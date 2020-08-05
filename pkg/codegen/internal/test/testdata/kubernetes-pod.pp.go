package main

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := core / v1.NewPod(ctx, "bar", &core/v1.PodArgs{
			ApiVersion: pulumi.String("v1"),
			Kind:       pulumi.String("Pod"),
			Metadata: &meta.ObjectMetaArgs{
				Namespace: pulumi.String("foo"),
				Name:      pulumi.String("bar"),
			},
			Spec: &core.PodSpecArgs{
				Containers: core.ContainerArray{
					&core.ContainerArgs{
						Name:  pulumi.String("nginx"),
						Image: pulumi.String("nginx:1.14-alpine"),
						Resources: &core.ResourceRequirementsArgs{
							Limits: pulumi.StringMap{
								"memory": pulumi.String("20Mi"),
								"cpu":    pulumi.String("0.2"),
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
