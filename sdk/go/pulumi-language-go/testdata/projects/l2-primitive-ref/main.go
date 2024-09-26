package main

import (
	"example.com/pulumi-primitive-ref/sdk/go/v11/primitiveref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := primitiveref.NewResource(ctx, "res", &primitiveref.ResourceArgs{
			Data: &primitiveref.DataArgs{
				Boolean: pulumi.Bool(false),
				Float:   pulumi.Float64(2.17),
				Integer: pulumi.Int(-12),
				String:  pulumi.String("Goodbye"),
				BoolArray: pulumi.BoolArray{
					pulumi.Bool(false),
					pulumi.Bool(true),
				},
				StringMap: pulumi.StringMap{
					"two":   pulumi.String("turtle doves"),
					"three": pulumi.String("french hens"),
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
