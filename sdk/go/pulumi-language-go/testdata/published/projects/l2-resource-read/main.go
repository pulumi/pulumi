package main

import (
	"example.com/pulumi-read/sdk/go/v39/read"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := read.GetResource(ctx, "res", pulumi.ID("existing-id"), &read.ResourceState{
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
