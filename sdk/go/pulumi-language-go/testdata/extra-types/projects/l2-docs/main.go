package main

import (
	"example.com/pulumi-docs/sdk/go/v25/docs"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := docs.NewResource(ctx, "res", &docs.ResourceArgs{
			In: docs.FunOutput(ctx, docs.FunOutputArgs{
				In: pulumi.Bool(false),
			}, nil).ApplyT(func(invoke docs.FunResult) (bool, error) {
				return invoke.Out, nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
