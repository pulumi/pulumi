package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := simple.NewResource(ctx, "res", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("name", pulumi.String(res.PulumiResourceName()))
		ctx.Export("type", pulumi.String(res.PulumiResourceType()))
		return nil
	})
}
