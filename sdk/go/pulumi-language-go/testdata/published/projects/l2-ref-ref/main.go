package main

import (
	"example.com/pulumi-ref-ref/sdk/go/v12/refref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := refref.NewResource(ctx, "res", &refref.ResourceArgs{
			Data: &refref.DataArgs{
				InnerData: &refref.InnerDataArgs{
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
				Boolean:   pulumi.Bool(true),
				Float:     pulumi.Float64(4.5),
				Integer:   pulumi.Int(1024),
				String:    pulumi.String("Hello"),
				BoolArray: pulumi.BoolArray{},
				StringMap: pulumi.StringMap{
					"x": pulumi.String("100"),
					"y": pulumi.String("200"),
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
