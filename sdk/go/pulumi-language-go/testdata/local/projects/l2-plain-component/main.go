package main

import (
	"example.com/pulumi-plaincomponent/sdk/go/v36/plaincomponent"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myComponent, err := plaincomponent.NewComponent(ctx, "myComponent", &plaincomponent.ComponentArgs{
			Name: "my-resource",
			Settings: plaincomponent.SettingsArgs{
				Enabled: true,
				Tags: map[string]string{
					"env": "test",
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("label", myComponent.Label)
		return nil
	})
}
