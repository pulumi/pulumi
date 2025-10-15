package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		target, err := simple.NewResource(ctx, "target", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "deletedWith", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.DeletedWith(target))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "notDeletedWith", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
