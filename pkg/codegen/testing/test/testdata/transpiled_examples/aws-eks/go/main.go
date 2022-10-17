package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/ec2"
	"github.com/pulumi/pulumi-eks/sdk/go/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		vpcId := ec2.LookupVpc(ctx, map[string]interface{}{
			"default": true,
		}, nil).Id
		subnetIds := ec2.GetSubnetIds(ctx, map[string]interface{}{
			"vpcId": vpcId,
		}, nil).Ids
		cluster, err := eks.NewCluster(ctx, "cluster", &eks.ClusterArgs{
			VpcId:           pulumi.String(vpcId),
			SubnetIds:       interface{}(subnetIds),
			InstanceType:    pulumi.String("t2.medium"),
			DesiredCapacity: pulumi.Int(2),
			MinSize:         pulumi.Int(1),
			MaxSize:         pulumi.Int(2),
		})
		if err != nil {
			return err
		}
		ctx.Export("kubeconfig", cluster.Kubeconfig)
		return nil
	})
}
