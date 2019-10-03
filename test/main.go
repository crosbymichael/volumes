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
	client, err := containerd.New(address)
	if err != nil {
		return err
	}
	defer client.Close()

	var (
		target = "/tmp/redis"
		volume = volumes.NewBindVolume(target, volumes.WithOptions([]string{"rw"}))
	)

	image, err := client.Pull(ctx, redis)
	if err != nil {
		return err
	}
	if err := image.UnpackTo(ctx, volumes.WithVolumeUnpack(volume)); err != nil {
		return err
	}
	container, err := client.NewContainer(
		ctx,
		id,
		containerd.WithNewSnapshot(id, image),
		// need to hack around withimageconfig because it uses the fs and snapshotters for
		// getting additional guids and usernames
		containerd.WithNewSpec(oci.WithRootFSPath(target), oci.WithImageConfig(image), oci.WithRootFSPath("rootfs")),
	)
	if err != nil {
		return err
	}
	// delete the snapshot with the container
	defer container.Delete(ctx)

	// use our stdio for the container's stdio
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio), volumes.WithVolume(volume))
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
