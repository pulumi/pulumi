package main

import (
	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := primitive.NewResource(ctx, "res", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(true),
			Float:   pulumi.Float64(3.14),
			Integer: pulumi.Int(42),
			String:  pulumi.String("hello"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(-1),
				pulumi.Float64(0),
				pulumi.Float64(1),
			},
			BooleanMap: pulumi.BoolMap{
				"t": pulumi.Bool(true),
				"f": pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
