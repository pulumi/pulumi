package main

import (
	"example.com/pulumi-simple-component/sdk/go/v15/simplecomponent"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simplecomponent.NewResource(ctx, "res", &simplecomponent.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
