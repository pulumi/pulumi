package main

import (
	"github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := eks.NewCluster(ctx, "rawkode", &eks.ClusterArgs{
			InstanceType:    pulumi.String("t2.medium"),
			DesiredCapacity: pulumi.Int(2),
			MinSize:         pulumi.Int(1),
			MaxSize:         pulumi.Int(2),
		})
		if err != nil {
			return err
		}
		_, err = eks.NewCluster(ctx, "stack72", &eks.ClusterArgs{
			InstanceType:    pulumi.String("t2.medium"),
			DesiredCapacity: pulumi.Int(4),
			MinSize:         pulumi.Int(1),
			MaxSize:         pulumi.Int(8),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
