package main

import (
	"example.com/pulumi-names/sdk/go/v6/names"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := names.NewResourceMapResource(ctx, "res2", &names.ResourceMapResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
