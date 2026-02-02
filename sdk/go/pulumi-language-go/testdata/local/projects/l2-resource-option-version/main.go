package main

import (
	"example.com/pulumi-simple/sdk/go/v26/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simple.NewResource(ctx, "withV2", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Version("2.0.0"))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withV26", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withDefault", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
