package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simple.NewResource(ctx, "hideDiffs", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.HideDiffs([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "notHideDiffs", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
