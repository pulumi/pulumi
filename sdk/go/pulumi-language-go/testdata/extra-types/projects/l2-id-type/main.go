package main

import (
	"encoding/base64"

	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Test that the ID type is treated the same as a string type, despite being tracked as a distinct type. This
		// includes directly passing it to string fields, but also for bool and numeric values being able to cast to it.
		source1, err := primitive.NewResource(ctx, "source1", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(false),
			Float:   pulumi.Float64(1),
			Integer: pulumi.Int(2),
			String:  pulumi.String("1234"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(3),
			},
			BooleanMap: pulumi.BoolMap{
				"source": pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		source2, err := primitive.NewResource(ctx, "source2", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(false),
			Float:   pulumi.Float64(1),
			Integer: pulumi.Int(2),
			String:  pulumi.String("true"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(3),
			},
			BooleanMap: pulumi.BoolMap{
				"source": pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		idMap := map[string]interface{}{
			"source1Token": source1.ID(),
			"source2Token": source2.ID(),
		}
		_, err = primitive.NewResource(ctx, "sink1", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(false),
			Float:   idMap["source1Token"].(string),
			Integer: idMap["source1Token"].(string),
			String:  idMap["source1Token"].(string),
			NumberArray: pulumi.Float64Array{
				idMap["source1Token"].(string),
			},
			BooleanMap: pulumi.BoolMap{
				"sink": pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		sink2, err := primitive.NewResource(ctx, "sink2", &primitive.ResourceArgs{
			Boolean: idMap["source2Token"].(string),
			Float:   pulumi.Float64(1),
			Integer: pulumi.Int(2),
			String:  pulumi.String("abc"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(3),
			},
			BooleanMap: pulumi.BoolMap{
				"sink": idMap["source2Token"].(string),
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("ids", idMap)
		ctx.Export("base64", sink2.ID().ApplyT(func(id string) (pulumi.String, error) {
			return pulumi.String(base64.StdEncoding.EncodeToString([]byte(id))), nil
		}).(pulumi.StringOutput))
		return nil
	})
}
