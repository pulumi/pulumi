package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res1, err := simple.NewResource(ctx, "res1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		localVar := res1.Value
		res2, err := simple.NewResource(ctx, "res2", &simple.ResourceArgs{
			Value: localVar,
		})
		if err != nil {
			return err
		}
		ctx.Export("out", res2.Value)
		return nil
	})
}
