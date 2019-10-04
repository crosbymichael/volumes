package main

import (
	"context"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/crosbymichael/volumes"
)

const (
	address = "/run/containerd/containerd.sock"
	redis   = "docker.io/library/redis:alpine"
	iqn     = "iqn.2019.com.crosbymichael.core:redis"
)

func main() {
	ctx := context.Background()
	// name your own namespace
	ctx = namespaces.WithNamespace(ctx, "mystuff")
	if err := runRedis(ctx, "redis"); err != nil {
		logrus.Error(err)
	}
}

func runRedis(ctx context.Context, id string) error {

	var (
		portal   = volumes.NewPortal("10.0.10.10", "3260")
		provider = volumes.New()
		vol      = volumes.NewBindVolume("/tmp/redis", volumes.WithOptions([]string{"rw"}))
	)
	target, err := portal.Target(ctx, iqn)
	if err != nil {
		return err
	}
	defer target.Logout(ctx)

	iscsiVol, err := volumes.NewIscsiVolume(ctx, target, 0, "ext4")
	if err != nil {
		return err
	}

	// add the bind mount
	provider.Add(ctx, id, vol)
	// add the iscsi volume
	provider.Add(ctx, iqn, iscsiVol)

	client, err := containerd.New(address, containerd.WithMountProvider(provider.ID(), provider))
	if err != nil {
		return err
	}
	defer client.Close()

	image, err := client.Pull(ctx, redis)
	if err != nil {
		return err
	}
	// unpack to the iscsi volume
	if err := image.UnpackTo(ctx, volumes.WithVolumeUnpack(iscsiVol)); err != nil {
		return err
	}
	container, err := client.NewContainer(
		ctx,
		id,
		// setup the volume id as the iqn this time
		volumes.WithVolumeRootfs(iqn, image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		return err
	}
	// delete the snapshot with the container
	defer container.Delete(ctx)

	// use our stdio for the container's stdio
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	// get the exit channel
	exit, err := task.Wait(ctx)
	if err != nil {
		return err
	}
	defer task.Delete(ctx)

	// start the task
	if err := task.Start(ctx); err != nil {
		return err
	}
	// for the example run for 3 sec then kill the redis container
	time.Sleep(30 * time.Second)
	if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
		return err
	}
	// wait for the task to exit
	<-exit
	return nil
}
