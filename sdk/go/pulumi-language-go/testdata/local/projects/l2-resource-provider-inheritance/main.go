package main

import (
	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		provider, err := simple.NewProvider(ctx, "provider", nil)
		if err != nil {
			return err
		}
		parent1, err := simple.NewResource(ctx, "parent1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}
		// This should inherit the explicit provider from parent1
		_, err = simple.NewResource(ctx, "child1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Parent(parent1))
		if err != nil {
			return err
		}
		parent2, err := primitive.NewResource(ctx, "parent2", &primitive.ResourceArgs{
			Boolean:     pulumi.Bool(false),
			Float:       pulumi.Float64(0),
			Integer:     pulumi.Int(0),
			String:      pulumi.String(""),
			NumberArray: pulumi.Float64Array{},
			BooleanMap:  pulumi.BoolMap{},
		})
		if err != nil {
			return err
		}
		// This _should not_ inherit the provider from parent2 as it is a default provider.
		_, err = simple.NewResource(ctx, "child2", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Parent(parent2))
		if err != nil {
			return err
		}
		return nil
	})
}
