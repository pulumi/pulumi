package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ecs"
	awsxecs "github.com/pulumi/pulumi-awsx/sdk/go/awsx/ecs"
	"github.com/pulumi/pulumi-awsx/sdk/go/awsx/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cluster, err := ecs.NewCluster(ctx, "cluster", nil)
		if err != nil {
			return err
		}
		lb, err := lb.NewApplicationLoadBalancer(ctx, "lb", nil)
		if err != nil {
			return err
		}
		_, err = awsxecs.NewFargateService(ctx, "nginx", &awsxecs.FargateServiceArgs{
			Cluster: cluster.Arn,
			TaskDefinitionArgs: &awsxecs.FargateServiceTaskDefinitionArgs{
				Container: &awsxecs.TaskDefinitionContainerDefinitionArgs{
					Image:  pulumi.String("nginx:latest"),
					Cpu:    pulumi.Int(512),
					Memory: pulumi.Int(128),
					PortMappings: awsxecs.TaskDefinitionPortMappingArray{
						&awsxecs.TaskDefinitionPortMappingArgs{
							ContainerPort: pulumi.Int(80),
							TargetGroup:   lb.DefaultTargetGroup,
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("url", lb.LoadBalancer.ApplyT(func(loadBalancer *lb.LoadBalancer) (string, error) {
			return loadBalancer.DnsName, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
