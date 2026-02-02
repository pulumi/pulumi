package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Stage 1: Change properties to trigger replacements
		// Resource with deleteBeforeReplace option - should delete before creating
		_, err := simple.NewResource(ctx, "withOption", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}
		// Resource without deleteBeforeReplace - should create before deleting (default)
		_, err = simple.NewResource(ctx, "withoutOption", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
