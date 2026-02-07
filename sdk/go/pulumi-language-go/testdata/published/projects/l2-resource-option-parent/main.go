package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		parent, err := simple.NewResource(ctx, "parent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withParent", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		}, pulumi.Parent(parent))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "noParent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
