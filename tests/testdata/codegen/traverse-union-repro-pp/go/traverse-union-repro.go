package main

import (
	"example.com/pulumi-infra/sdk/go/infra"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := infra.NewFileSystem(ctx, "test", &infra.FileSystemArgs{
			StorageCapacity: pulumi.Int(64),
			SubnetIds: pulumi.String{
				aws_subnet.Test1.Id,
			},
			DeploymentType:     pulumi.String("SINGLE_AZ_1"),
			ThroughputCapacity: pulumi.Int(64),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
