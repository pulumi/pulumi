package main

import (
	"example.com/pulumi-discriminated-union/sdk/go/v31/discriminatedunion"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := discriminatedunion.NewExample(ctx, "example1", &discriminatedunion.ExampleArgs{
			UnionOf: &discriminatedunion.VariantOneArgs{
				DiscriminantKind: pulumi.String("variant1"),
				Field1:           pulumi.String("v1 union"),
			},
			ArrayOfUnionOf: pulumi.Array{
				&discriminatedunion.VariantOneArgs{
					DiscriminantKind: pulumi.String("variant1"),
					Field1:           pulumi.String("v1 array(union)"),
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunion.NewExample(ctx, "example2", &discriminatedunion.ExampleArgs{
			UnionOf: &discriminatedunion.VariantTwoArgs{
				DiscriminantKind: pulumi.String("variant2"),
				Field2:           pulumi.String("v2 union"),
			},
			ArrayOfUnionOf: pulumi.Array{
				&discriminatedunion.VariantTwoArgs{
					DiscriminantKind: pulumi.String("variant2"),
					Field2:           pulumi.String("v2 array(union)"),
				},
				&discriminatedunion.VariantOneArgs{
					DiscriminantKind: pulumi.String("variant1"),
					Field1:           pulumi.String("v1 array(union)"),
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
