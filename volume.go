package volumes

import (
	"github.com/opencontainers/runtime-spec/specs-go"
)

type MountOpts func(options *[]string)

type VolumeType int

const (
	Iscsi VolumeType = iota + 1
	Cifs
)

type Volume interface {
	ID() string
	Type() VolumeType
	OCIMount(string) specs.Mount
	Mount(string) error
}

func WithOptions(o []string) MountOpts {
	return func(options *[]string) {
		*options = append(*options, o...)
	}
}
