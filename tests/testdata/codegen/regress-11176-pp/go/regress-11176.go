package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ecs"
	awsxecs "github.com/pulumi/pulumi-awsx/sdk/go/awsx/ecs"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cluster, err := ecs.NewCluster(ctx, "cluster", nil)
		if err != nil {
			return err
		}
		_, err = awsxecs.NewFargateService(ctx, "nginx", &awsxecs.FargateServiceArgs{
			Cluster: cluster.Arn,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
