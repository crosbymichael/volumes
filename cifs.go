package volumes

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func NewCifsVolume(source string, opts ...MountOpts) Volume {
	v := &cifsVolume{
		source: source,
	}
	for _, o := range opts {
		o(&v.options)
	}
	return v
}

type cifsVolume struct {
	source  string
	options []string
}

func (c *cifsVolume) OCIMount(dest string) specs.Mount {
	return specs.Mount{
		Type:        "cifs",
		Source:      c.source,
		Destination: dest,
		Options:     c.options,
	}
}

func (c *cifsVolume) Mount(dest string) error {
	flags, data := parseMountOptions(c.options)
	return unix.Mount(c.source, dest, "cifs", uintptr(flags), data)
}

func (c *cifsVolume) Mounts(ctx context.Context) ([]mount.Mount, error) {
	return []mount.Mount{
		{
			Type:    "cifs",
			Source:  c.source,
			Options: c.options,
		},
	}, nil
}

func WithUsernameAndPassword(username, password string) MountOpts {
	return func(options *[]string) {
		*options = append(*options, fmt.Sprintf("username=%s", username))
		if password != "" {
			*options = append(*options, fmt.Sprintf("password=%s", password))
		}
	}
}

func WithUIDGID(uid, gid int) MountOpts {
	return func(options *[]string) {
		*options = append(*options, fmt.Sprintf("uid=%d", uid), fmt.Sprintf("gid=%d", gid))
	}
}
