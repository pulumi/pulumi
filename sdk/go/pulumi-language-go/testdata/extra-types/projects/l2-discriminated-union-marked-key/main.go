package main

import (
	"example.com/pulumi-discriminated-union-marked-key/sdk/go/v53/discriminatedunionmarkedkey"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		first, err := discriminatedunionmarkedkey.NewExample(ctx, "first", &discriminatedunionmarkedkey.ExampleArgs{
			UnionIn: &discriminatedunionmarkedkey.VariantTwoArgs{
				DiscriminantKind: pulumi.String("variant2"),
				Field2:           pulumi.String("known"),
			},
		})
		if err != nil {
			return err
		}
		second, err := discriminatedunionmarkedkey.NewExample(ctx, "second", &discriminatedunionmarkedkey.ExampleArgs{
			UnionIn: first.UnionOut,
		})
		if err != nil {
			return err
		}
		ctx.Export("out", second.UnionOut)
		return nil
	})
}
