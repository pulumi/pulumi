package main

import (
	"example.com/pulumi-read-id-rewrite/sdk/go/v58/readidrewrite"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := readidrewrite.GetResource(ctx, "res", pulumi.ID("existing-id"), &readidrewrite.ResourceState{
			Lookup: pulumi.String("existing-key"),
		})
		if err != nil {
			return err
		}
		src, err := simple.NewResource(ctx, "src", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		unknownRes, err := readidrewrite.GetResource(ctx, "unknownRes", src.ID(), &readidrewrite.ResourceState{
			Lookup: pulumi.String("another-key"),
		})
		if err != nil {
			return err
		}
		ctx.Export("resourceId", res.ID())
		ctx.Export("lookup", res.Lookup)
		ctx.Export("unknownResourceId", unknownRes.ID())
		ctx.Export("unknownLookup", unknownRes.Lookup)
		return nil
	})
}
