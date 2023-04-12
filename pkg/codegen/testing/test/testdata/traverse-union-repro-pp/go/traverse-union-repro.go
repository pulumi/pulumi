package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/fsx"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := fsx.NewOpenZfsFileSystem(ctx, "test", &fsx.OpenZfsFileSystemArgs{
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
