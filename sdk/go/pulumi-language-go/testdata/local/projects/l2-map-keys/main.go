package main

import (
	"example.com/pulumi-plain/sdk/go/v13/plain"
	"example.com/pulumi-primitive-ref/sdk/go/v11/primitiveref"
	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"example.com/pulumi-ref-ref/sdk/go/v12/refref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := primitive.NewResource(ctx, "prim", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(false),
			Float:   pulumi.Float64(2.17),
			Integer: pulumi.Int(-12),
			String:  pulumi.String("Goodbye"),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(0),
				pulumi.Float64(1),
			},
			BooleanMap: pulumi.BoolMap{
				"my key": pulumi.Bool(false),
				"my.key": pulumi.Bool(true),
				"my-key": pulumi.Bool(false),
				"my_key": pulumi.Bool(true),
				"MY_KEY": pulumi.Bool(false),
				"myKey":  pulumi.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		_, err = primitiveref.NewResource(ctx, "ref", &primitiveref.ResourceArgs{
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
					"my key": pulumi.String("one"),
					"my.key": pulumi.String("two"),
					"my-key": pulumi.String("three"),
					"my_key": pulumi.String("four"),
					"MY_KEY": pulumi.String("five"),
					"myKey":  pulumi.String("six"),
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = refref.NewResource(ctx, "rref", &refref.ResourceArgs{
			Data: &refref.DataArgs{
				InnerData: &refref.InnerDataArgs{
					Boolean:   pulumi.Bool(false),
					Float:     pulumi.Float64(-2.17),
					Integer:   pulumi.Int(123),
					String:    pulumi.String("Goodbye"),
					BoolArray: pulumi.BoolArray{},
					StringMap: pulumi.StringMap{
						"my key": pulumi.String("one"),
						"my.key": pulumi.String("two"),
						"my-key": pulumi.String("three"),
						"my_key": pulumi.String("four"),
						"MY_KEY": pulumi.String("five"),
						"myKey":  pulumi.String("six"),
					},
				},
				Boolean:   pulumi.Bool(true),
				Float:     pulumi.Float64(4.5),
				Integer:   pulumi.Int(1024),
				String:    pulumi.String("Hello"),
				BoolArray: pulumi.BoolArray{},
				StringMap: pulumi.StringMap{
					"my key": pulumi.String("one"),
					"my.key": pulumi.String("two"),
					"my-key": pulumi.String("three"),
					"my_key": pulumi.String("four"),
					"MY_KEY": pulumi.String("five"),
					"myKey":  pulumi.String("six"),
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = plain.NewResource(ctx, "plains", &plain.ResourceArgs{
			Data: plain.DataArgs{
				InnerData: plain.InnerDataArgs{
					Boolean: false,
					Float:   2.17,
					Integer: -12,
					String:  "Goodbye",
					BoolArray: []bool{
						false,
						true,
					},
					StringMap: map[string]string{
						"my key": "one",
						"my.key": "two",
						"my-key": "three",
						"my_key": "four",
						"MY_KEY": "five",
						"myKey":  "six",
					},
				},
				Boolean: true,
				Float:   4.5,
				Integer: 1024,
				String:  "Hello",
				BoolArray: []bool{
					true,
					false,
				},
				StringMap: map[string]string{
					"my key": "one",
					"my.key": "two",
					"my-key": "three",
					"my_key": "four",
					"MY_KEY": "five",
					"myKey":  "six",
				},
			},
			NonPlainData: &plain.DataArgs{
				InnerData: plain.InnerDataArgs{
					Boolean: false,
					Float:   2.17,
					Integer: -12,
					String:  "Goodbye",
					BoolArray: []bool{
						false,
						true,
					},
					StringMap: map[string]interface{}{
						"my key": "one",
						"my.key": "two",
						"my-key": "three",
						"my_key": "four",
						"MY_KEY": "five",
						"myKey":  "six",
					},
				},
				Boolean: true,
				Float:   4.5,
				Integer: 1024,
				String:  "Hello",
				BoolArray: []bool{
					true,
					false,
				},
				StringMap: map[string]interface{}{
					"my key": "one",
					"my.key": "two",
					"my-key": "three",
					"my_key": "four",
					"MY_KEY": "five",
					"myKey":  "six",
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
