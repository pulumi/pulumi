package main

import (
	"example.com/pulumi-union/sdk/go/union"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := union.NewExample(ctx, "stringOrIntegerExample1", &union.ExampleArgs{
			StringOrIntegerProperty: pulumi.Any(42),
		})
		if err != nil {
			return err
		}
		_, err = union.NewExample(ctx, "stringOrIntegerExample2", &union.ExampleArgs{
			StringOrIntegerProperty: pulumi.Any("forty two"),
		})
		if err != nil {
			return err
		}
		mapMapUnionExample, err := union.NewExample(ctx, "mapMapUnionExample", &union.ExampleArgs{
			MapMapUnionProperty: pulumi.MapMap{
				"key1": pulumi.Map{
					"key1a": pulumi.Any("value1a"),
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("mapMapUnionOutput", mapMapUnionExample.MapMapUnionProperty)
		return nil
	})
}
