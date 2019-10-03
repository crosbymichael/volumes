package volumes

import (
	"github.com/containerd/containerd/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func NewBindVolume(source string, opts ...MountOpts) Volume {
	v := &bindVolume{
		source: source,
		options: []string{
			"bind",
		},
	}
	for _, o := range opts {
		o(&v.options)
	}
	return v
}

type bindVolume struct {
	source  string
	options []string
}

func (b *bindVolume) Type() VolumeType {
	return Bind
}

func (b *bindVolume) OCIMount(dest string) specs.Mount {
	return specs.Mount{
		Type:        "bind",
		Source:      b.source,
		Destination: dest,
		Options:     b.options,
	}
}

func (b *bindVolume) Mount(dest string) error {
	flags, data := parseMountOptions(b.options)
	return unix.Mount(b.source, dest, "none", uintptr(flags), data)
}

func (b *bindVolume) Mounts() []mount.Mount {
	return []mount.Mount{
		{
			Type:    "bind",
			Source:  b.source,
			Options: b.options,
		},
	}
}
