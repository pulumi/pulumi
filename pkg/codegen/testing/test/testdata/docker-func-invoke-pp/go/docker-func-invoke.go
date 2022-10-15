package main

import (
	"github.com/pulumi/pulumi-docker/sdk/v3/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		latest, err := docker.LookupRemoteImage(ctx, &docker.LookupRemoteImageArgs{
			Name: "nginx",
		}, nil)
		if err != nil {
			return err
		}
		ubuntu, err := docker.NewRemoteImage(ctx, "ubuntu", &docker.RemoteImageArgs{
			Name: pulumi.String("ubuntu:precise"),
		})
		if err != nil {
			return err
		}
		ctx.Export("remoteImageId", pulumi.String(latest.Id))
		ctx.Export("ubuntuImage", ubuntu.Name)
		return nil
	})
}
