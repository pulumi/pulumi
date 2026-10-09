package main

import (
	"example.com/pulumi-read/sdk/go/v39/read"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		failing, err := read.GetResource(ctx, "failing", pulumi.ID("existing-id"), &read.ResourceState{
			Lookup: pulumi.String("fail"),
		})
		if err != nil {
			return err
		}
		ctx.Export("failingValue", failing.Value)
		ctx.Export("failingLookup", failing.Lookup)
		return nil
	})
}
