package volumes

import (
	"context"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

const providerID = "com.crosbymichael.volumes.provider.v1"

func New() *VolumeProvider {
	return &VolumeProvider{
		volumes: make(map[string]Volume),
	}
}

type VolumeProvider struct {
	mu      sync.Mutex
	volumes map[string]Volume
}

func (v *VolumeProvider) ID() string {
	return providerID
}

func (b *VolumeProvider) Add(ctx context.Context, key string, v Volume) {
	b.mu.Lock()
	b.volumes[key] = v
	b.mu.Unlock()
}

func (b *VolumeProvider) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	b.mu.Lock()
	v, ok := b.volumes[key]
	b.mu.Unlock()
	if !ok {
		return nil, errors.Wrap(errdefs.ErrNotFound, "volume does not exist")
	}
	return v.Mounts(ctx)
}

type MountOpts func(options *[]string)

type Volume interface {
	OCIMount(string) specs.Mount
	Mount(string) error
	Mounts(context.Context) ([]mount.Mount, error)
}

func WithOptions(o []string) MountOpts {
	return func(options *[]string) {
		*options = append(*options, o...)
	}
}

func WithVolumeUnpack(v Volume) containerd.UnpackOpt {
	return func(ctx context.Context, c *containerd.UnpackConfig) error {
		mounts, err := v.Mounts(ctx)
		if err != nil {
			return err
		}
		c.Mounts = mounts
		return nil
	}
}

func WithVolumeRootfs(id string, i containerd.Image, opts ...snapshots.Opt) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		c.Snapshotter = providerID
		c.SnapshotKey = id
		c.Image = i.Name()
		return nil
	}
}
