package volumes

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func NewIscsiVolume(ctx context.Context, t *Target, lun int, fstype string, opts ...MountOpts) (Volume, error) {
	v := &iscsiVolume{
		target: t,
		lun:    lun,
		fstype: fstype,
	}
	for _, o := range opts {
		o(&v.options)
	}
	if err := t.Login(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

type iscsiVolume struct {
	target  *Target
	lun     int
	fstype  string
	options []string
}

func (i *iscsiVolume) OCIMount(dest string) specs.Mount {
	return specs.Mount{
		Type:        i.fstype,
		Source:      i.target.path(i.lun),
		Destination: dest,
		Options:     i.options,
	}
}

func (i *iscsiVolume) Mount(dest string) error {
	flags, data := parseMountOptions(i.options)
	return unix.Mount(i.target.path(i.lun), dest, i.fstype, uintptr(flags), data)
}

func (i *iscsiVolume) Mounts(ctx context.Context) ([]mount.Mount, error) {
	if err := i.target.Login(ctx); err != nil {
		return nil, err
	}
	return []mount.Mount{
		{
			Type:    i.fstype,
			Source:  i.target.path(i.lun),
			Options: i.options,
		},
	}, nil
}

const defaultLUN = 0

type Target struct {
	IQN    string
	Portal *Portal
}

// Login to the target
func (t *Target) Login(ctx context.Context) error {
	if _, err := adm(ctx,
		"--mode", "node",
		"--targetname", t.IQN,
		"--portal", t.Portal.address,
		"--login"); err != nil {
		if isPresentErr(err) {
			return nil
		}
		return errors.Wrap(err, "login")
	}
	// wait for the lun to exist on the node
	return t.ready(ctx)
}

// Logout from the target
func (t *Target) Logout(ctx context.Context) error {
	if _, err := adm(ctx,
		"--mode", "node",
		"--targetname", t.IQN,
		"--portal", t.Portal.address,
		"--logout"); err != nil && !isNotSessionErr(err) {
		return errors.Wrap(err, "logout")
	}
	return nil
}

// ready polls for the device node to until it exists or the
// context cancels
func (t *Target) ready(ctx context.Context) error {
	path := t.path(defaultLUN)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if _, err := os.Lstat(path); err == nil {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (t *Target) path(lun int) string {
	return filepath.Join("/dev/disk/by-path", fmt.Sprintf(
		"ip-%s-iscsi-%s-lun-%d", t.Portal.address, t.IQN, lun),
	)
}

// iscsi default port
const defaultPort = "3260"

func NewPortal(ip, port string) *Portal {
	if port == "" {
		port = defaultPort
	}
	return &Portal{
		address: net.JoinHostPort(ip, port),
	}
}

// Portal is an iscsi portal serving targets
type Portal struct {
	address string
}

func (p *Portal) Target(ctx context.Context, iqn string) (*Target, error) {
	targets, err := p.Targets(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		if t.IQN == iqn {
			return t, nil
		}
	}
	return nil, errors.Wrapf(os.ErrNotExist, "%s does not exist in portal", iqn)
}

// Targets returns all targets for the portal
func (p *Portal) Targets(ctx context.Context) ([]*Target, error) {
	out, err := adm(ctx,
		"--mode", "discovery",
		"-t", "sendtargets",
		"--portal", p.address)
	if err != nil {
		return nil, errors.Wrap(err, "discover targets")
	}
	return parseTargets(p, out)
}

// adm runs iscsiadm commands
func adm(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "iscsiadm", args...).CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "%s", out)
	}
	return out, nil
}

func isPresentErr(err error) bool {
	return strings.Contains(err.Error(), "already present")
}

func isNotSessionErr(err error) bool {
	return strings.Contains(err.Error(), "No matching sessions found")
}

func parseTargets(p *Portal, data []byte) ([]*Target, error) {
	var out []*Target
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}
		text := s.Text()
		if text == "" {
			continue
		}
		parts := strings.SplitN(text, " ", 2)
		out = append(out, &Target{
			IQN:    parts[1],
			Portal: p,
		})
	}
	return out, nil
}

func WithTargetLogout(t *Target) containerd.ProcessDeleteOpts {
	return func(ctx context.Context, p containerd.Process) error {
		return t.Logout(ctx)
	}
}
