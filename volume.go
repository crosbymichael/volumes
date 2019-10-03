package volumes

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type MountOpts func(options *[]string)

type VolumeType int

const (
	Iscsi VolumeType = iota + 1
	Cifs
	Bind
)

type Volume interface {
	Type() VolumeType
	OCIMount(string) specs.Mount
	Mount(string) error
	Mounts() []mount.Mount
}

func WithOptions(o []string) MountOpts {
	return func(options *[]string) {
		*options = append(*options, o...)
	}
}

func WithVolumeUnpack(v Volume) containerd.UnpackOpt {
	return func(ctx context.Context, c *containerd.UnpackConfig) error {
		c.Mounts = v.Mounts()
		return nil
	}
}

func WithVolume(v Volume) containerd.NewTaskOpts {
	return func(ctx context.Context, c *containerd.Client, i *containerd.TaskInfo) error {
		i.RootFS = v.Mounts()
		return nil
	}
}
