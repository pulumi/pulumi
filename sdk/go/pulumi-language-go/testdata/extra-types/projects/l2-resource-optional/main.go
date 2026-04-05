package main

import (
	"example.com/pulumi-optionalprimitive/sdk/go/v34/optionalprimitive"
	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		unsetA, err := optionalprimitive.NewResource(ctx, "unsetA", nil)
		if err != nil {
			return err
		}
		unsetB, err := optionalprimitive.NewResource(ctx, "unsetB", &optionalprimitive.ResourceArgs{
			Boolean:     unsetA.Boolean,
			Float:       unsetA.Float,
			Integer:     unsetA.Integer,
			String:      unsetA.String,
			NumberArray: unsetA.NumberArray,
			BooleanMap:  unsetA.BooleanMap,
		})
		if err != nil {
			return err
		}
		ctx.Export("unsetBoolean", unsetB.Boolean.ApplyT(func(boolean *bool) (string, error) {
			var tmp0 string
			if boolean == nil {
				tmp0 = "null"
			} else {
				tmp0 = "not null"
			}
			return tmp0, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetFloat", unsetB.Float.ApplyT(func(float *float64) (string, error) {
			var tmp1 string
			if float == nil {
				tmp1 = "null"
			} else {
				tmp1 = "not null"
			}
			return tmp1, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetInteger", unsetB.Integer.ApplyT(func(integer *int) (string, error) {
			var tmp2 string
			if integer == nil {
				tmp2 = "null"
			} else {
				tmp2 = "not null"
			}
			return tmp2, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetString", unsetB.String.ApplyT(func(_string *string) (string, error) {
			var tmp3 string
			if _string == nil {
				tmp3 = "null"
			} else {
				tmp3 = "not null"
			}
			return tmp3, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetNumberArray", unsetB.NumberArray.ApplyT(func(numberArray []float64) (string, error) {
			var tmp4 string
			if numberArray == nil {
				tmp4 = "null"
			} else {
				tmp4 = "not null"
			}
			return tmp4, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetBooleanMap", unsetB.BooleanMap.ApplyT(func(booleanMap map[string]bool) (string, error) {
			var tmp5 string
			if booleanMap == nil {
				tmp5 = "null"
			} else {
				tmp5 = "not null"
			}
			return tmp5, nil
		}).(pulumi.StringOutput))
		setA, err := optionalprimitive.NewResource(ctx, "setA", &optionalprimitive.ResourceArgs{
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
		setB, err := optionalprimitive.NewResource(ctx, "setB", &optionalprimitive.ResourceArgs{
			Boolean:     setA.Boolean,
			Float:       setA.Float,
			Integer:     setA.Integer,
			String:      setA.String,
			NumberArray: setA.NumberArray,
			BooleanMap:  setA.BooleanMap,
		})
		if err != nil {
			return err
		}
		sourcePrimitive, err := primitive.NewResource(ctx, "sourcePrimitive", &primitive.ResourceArgs{
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
		_, err = optionalprimitive.NewResource(ctx, "fromPrimitive", &optionalprimitive.ResourceArgs{
			Boolean:     sourcePrimitive.Boolean,
			Float:       sourcePrimitive.Float,
			Integer:     sourcePrimitive.Integer,
			String:      sourcePrimitive.String,
			NumberArray: sourcePrimitive.NumberArray,
			BooleanMap:  sourcePrimitive.BooleanMap,
		})
		if err != nil {
			return err
		}
		ctx.Export("setBoolean", setB.Boolean)
		ctx.Export("setFloat", setB.Float)
		ctx.Export("setInteger", setB.Integer)
		ctx.Export("setString", setB.String)
		ctx.Export("setNumberArray", setB.NumberArray)
		ctx.Export("setBooleanMap", setB.BooleanMap)
		return nil
	})
}
