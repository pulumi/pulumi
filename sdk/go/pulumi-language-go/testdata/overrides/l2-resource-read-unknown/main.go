package main

import (
	"example.com/pulumi-read/sdk/go/v39/read"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		src, err := read.NewResource(ctx, "src", &read.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		res, err := read.GetResource(ctx, "res", src.ID(), &read.ResourceState{
			Lookup: pulumi.String("existing-key"),
		})
		if err != nil {
			return err
		}
		ctx.Export("resourceId", res.ID())
		ctx.Export("resourceUrn", res.URN())
		ctx.Export("lookup", res.Lookup)
		ctx.Export("value", res.Value)
		return nil
	})
}
