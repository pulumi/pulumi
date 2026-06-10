package main

import (
	"example.com/pulumi-manifest/sdk/go/v43/manifest"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		first, err := manifest.NewResource(ctx, "first", &manifest.ResourceArgs{
			Kind: pulumi.String("Manifest"),
			Metadata: &manifest.MetadataArgs{
				Name: pulumi.String("first"),
				Labels: pulumi.StringMap{
					"app": pulumi.String("first"),
				},
			},
			Spec: &manifest.SpecArgs{
				Replicas: pulumi.Int(1),
				Template: &manifest.TemplateArgs{
					Metadata: &manifest.MetadataArgs{
						Name: pulumi.String("inner"),
					},
					Containers: manifest.ContainerArray{
						&manifest.ContainerArgs{
							Name:  pulumi.String("app"),
							Image: pulumi.String("nginx"),
							Ports: pulumi.IntArray{
								pulumi.Int(80),
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("kind", first.Kind)
		return nil
	})
}
