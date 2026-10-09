package main

import (
	"example.com/pulumi-fail_on_create/sdk/go/v4/fail_on_create"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		failing, err := fail_on_create.NewResource(ctx, "failing", &fail_on_create.ResourceArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "independent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("failingValue", failing.Value)
		return nil
	})
}
