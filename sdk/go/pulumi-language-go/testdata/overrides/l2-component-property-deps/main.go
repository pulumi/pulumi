package main

import (
	"example.com/pulumi-component-property-deps/sdk/go/componentpropertydeps"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		custom1, err := componentpropertydeps.NewCustom(ctx, "custom1", &componentpropertydeps.CustomArgs{
			Value: pulumi.String("hello"),
		})
		if err != nil {
			return err
		}
		custom2, err := componentpropertydeps.NewCustom(ctx, "custom2", &componentpropertydeps.CustomArgs{
			Value: pulumi.String("world"),
		})
		if err != nil {
			return err
		}
		component1, err := componentpropertydeps.NewComponent(ctx, "component1", &componentpropertydeps.ComponentArgs{
			Resource: custom1,
			ResourceList: []*componentpropertydeps.Custom{
				custom1,
				custom2,
			},
			ResourceMap: map[string]*componentpropertydeps.Custom{
				"one": custom1,
				"two": custom2,
			},
		})
		if err != nil {
			return err
		}
		result, err := component1.Refs(ctx, &componentpropertydeps.ComponentRefsArgs{
			Resource:     custom1,
			ResourceList: []componentpropertydeps.CustomInput{custom1, custom2},
			ResourceMap: map[string]componentpropertydeps.CustomInput{
				"one": custom1,
				"two": custom2,
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("propertyDepsFromCall", result.Result())
		return nil
	})
}
