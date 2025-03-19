package main

import (
	"github.com/a-namespace/pulumi-namespaced/sdk/go/v16/namespaced"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := namespaced.NewResource(ctx, "res", &namespaced.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
