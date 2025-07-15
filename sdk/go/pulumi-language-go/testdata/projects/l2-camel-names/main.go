package main

import (
	"example.com/pulumi-camelNames/sdk/go/v19/camelNames/coolmodule"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		firstResource, err := coolmodule.NewSomeResource(ctx, "firstResource", &coolmodule.SomeResourceArgs{
			TheInput: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = coolmodule.NewSomeResource(ctx, "secondResource", &coolmodule.SomeResourceArgs{
			TheInput: firstResource.TheOutput,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
