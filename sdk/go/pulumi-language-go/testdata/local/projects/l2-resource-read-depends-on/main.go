package main

import (
	"example.com/pulumi-read/sdk/go/v39/read"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		src, err := simple.NewResource(ctx, "src", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = read.GetResource(ctx, "res", pulumi.ID("existing-id"), &read.ResourceState{
			Lookup: pulumi.String("existing-key"),
		}, pulumi.DependsOn([]pulumi.Resource{
			src,
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
