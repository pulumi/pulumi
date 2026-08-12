package main

import (
	"example.com/pulumi-alpha/sdk/go/v3/alpha"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := alpha.NewResource(ctx, "res", &alpha.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
