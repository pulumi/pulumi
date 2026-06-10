package main

import (
	"example.com/pulumi-conflicta/sdk/go/conflicta/mod"
	conflictbmod "example.com/pulumi-conflictb/sdk/go/conflictb/mod"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cluster, err := mod.NewCluster(ctx, "cluster", nil)
		if err != nil {
			return err
		}
		_, err = conflictbmod.NewService(ctx, "nginx", &conflictbmod.ServiceArgs{
			Cluster: cluster.Arn,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
