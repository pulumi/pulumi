package main

import (
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
		_, err = simple.NewResource(ctx, "child1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Parent(parent1))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "orphan1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		parent2, err := simple.NewResource(ctx, "parent2", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "child2", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Parent(parent2))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "child3", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Parent(parent2), pulumi.Protect(false))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "orphan2", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
