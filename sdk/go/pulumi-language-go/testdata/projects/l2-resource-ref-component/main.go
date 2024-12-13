package main

import (
	"example.com/pulumi-ref-component/sdk/go/v16/refcomponent"
	"example.com/pulumi-ref-ref/sdk/go/v12/refref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := refcomponent.NewResource(ctx, "res", &refcomponent.ResourceArgs{
			Inputs: &refref.InnerDataArgs{
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
