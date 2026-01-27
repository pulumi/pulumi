package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Stage 0: Initial resource creation
		// Resource with deleteBeforeReplace option
		_, err := simple.NewResource(ctx, "withOption", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}
		// Resource without deleteBeforeReplace (default create-before-delete behavior)
		_, err = simple.NewResource(ctx, "withoutOption", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplaceOnChanges([]string{
			"value",
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
