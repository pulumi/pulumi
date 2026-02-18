package main

import (
	"example.com/pulumi-union/sdk/go/v18/union"
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
		// List<Union<String, Enum>> pattern
		_, err = union.NewExample(ctx, "stringEnumUnionListExample", &union.ExampleArgs{
			StringEnumUnionListProperty: pulumi.StringArray{
				pulumi.String(union.AccessRightsListen),
				pulumi.String(union.AccessRightsSend),
				pulumi.String("NotAnEnumValue"),
			},
		})
		if err != nil {
			return err
		}
		// Safe enum: literal string matching an enum value
		_, err = union.NewExample(ctx, "safeEnumExample", &union.ExampleArgs{
			TypedEnumProperty: pulumi.String(union.BlobTypeBlock),
		})
		if err != nil {
			return err
		}
		// Output enum: output from another resource used as enum input
		enumOutputExample, err := union.NewEnumOutput(ctx, "enumOutputExample", &union.EnumOutputArgs{
			Name: pulumi.String("example"),
		})
		if err != nil {
			return err
		}
		_, err = union.NewExample(ctx, "outputEnumExample", &union.ExampleArgs{
			TypedEnumProperty: enumOutputExample.Type,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
