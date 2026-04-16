package main

import (
	"example.com/pulumi-ref-ref/sdk/go/v12/refref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Check we can index into properties of objects returned in outputs, this is similar to ref-ref but
		// we index into the outputs
		res, err := refref.NewResource(ctx, "res", &refref.ResourceArgs{
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
				Boolean: pulumi.Bool(true),
				Float:   pulumi.Float64(4.5),
				Integer: pulumi.Int(1024),
				String:  pulumi.String("Hello"),
				BoolArray: pulumi.BoolArray{
					pulumi.Bool(true),
				},
				StringMap: pulumi.StringMap{
					"x": pulumi.String("100"),
					"y": pulumi.String("200"),
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("bool", res.Data.ApplyT(func(data refref.Data) (bool, error) {
			return data.Boolean, nil
		}).(pulumi.BoolOutput))
		ctx.Export("array", res.Data.ApplyT(func(data refref.Data) (bool, error) {
			return data.BoolArray[0], nil
		}).(pulumi.BoolOutput))
		ctx.Export("map", res.Data.ApplyT(func(data refref.Data) (string, error) {
			return data.StringMap["x"], nil
		}).(pulumi.StringOutput))
		ctx.Export("nested", res.Data.ApplyT(func(data refref.Data) (string, error) {
			return data.InnerData.StringMap["three"], nil
		}).(pulumi.StringOutput))
		return nil
	})
}
