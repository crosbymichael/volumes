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
	var (
		provider = volumes.New()
		vol      = volumes.NewBindVolume("/tmp/redis", volumes.WithOptions([]string{"rw"}))
	)

	provider.Add(ctx, id, vol)

	client, err := containerd.New(address, containerd.WithMountProvider(provider.ID(), provider))
	if err != nil {
		return err
	}
	defer client.Close()

	image, err := client.Pull(ctx, redis)
	if err != nil {
		return err
	}
	if err := image.UnpackTo(ctx, volumes.WithVolumeUnpack(vol)); err != nil {
		return err
	}
	container, err := client.NewContainer(
		ctx,
		id,
		volumes.WithVolumeRootfs(id, image),
		// need to hack around withimageconfig because it uses the fs and snapshotters for
		// getting additional guids and usernames
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
