package main

import (
	"example.com/pulumi-optional-primitive-ref/sdk/go/v40/optionalprimitiveref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		setRes, err := optionalprimitiveref.NewResource(ctx, "setRes", &optionalprimitiveref.ResourceArgs{
			Data: &optionalprimitiveref.DataArgs{
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
			},
		})
		if err != nil {
			return err
		}
		unsetRes, err := optionalprimitiveref.NewResource(ctx, "unsetRes", &optionalprimitiveref.ResourceArgs{
			Data: &optionalprimitiveref.DataArgs{},
		})
		if err != nil {
			return err
		}
		ctx.Export("setBoolean", setRes.Data.ApplyT(func(data optionalprimitiveref.Data) (*bool, error) {
			return data.Boolean, nil
		}).(pulumi.BoolPtrOutput))
		ctx.Export("setFloat", setRes.Data.ApplyT(func(data optionalprimitiveref.Data) (*float64, error) {
			return data.Float, nil
		}).(pulumi.Float64PtrOutput))
		ctx.Export("setInteger", setRes.Data.ApplyT(func(data optionalprimitiveref.Data) (*int, error) {
			return data.Integer, nil
		}).(pulumi.IntPtrOutput))
		ctx.Export("setString", setRes.Data.ApplyT(func(data optionalprimitiveref.Data) (*string, error) {
			return data.String, nil
		}).(pulumi.StringPtrOutput))
		ctx.Export("setNumberArray", setRes.Data.ApplyT(func(data optionalprimitiveref.Data) ([]float64, error) {
			return data.NumberArray, nil
		}).(pulumi.Float64ArrayOutput))
		ctx.Export("setBooleanMap", setRes.Data.ApplyT(func(data optionalprimitiveref.Data) (map[string]bool, error) {
			return data.BooleanMap, nil
		}).(pulumi.BoolMapOutput))
		ctx.Export("unsetBoolean", unsetRes.Data.ApplyT(func(data optionalprimitiveref.Data) (string, error) {
			var tmp0 string
			if data.Boolean == nil {
				tmp0 = "null"
			} else {
				tmp0 = "not null"
			}
			return tmp0, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetFloat", unsetRes.Data.ApplyT(func(data optionalprimitiveref.Data) (string, error) {
			var tmp1 string
			if data.Float == nil {
				tmp1 = "null"
			} else {
				tmp1 = "not null"
			}
			return tmp1, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetInteger", unsetRes.Data.ApplyT(func(data optionalprimitiveref.Data) (string, error) {
			var tmp2 string
			if data.Integer == nil {
				tmp2 = "null"
			} else {
				tmp2 = "not null"
			}
			return tmp2, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetString", unsetRes.Data.ApplyT(func(data optionalprimitiveref.Data) (string, error) {
			var tmp3 string
			if data.String == nil {
				tmp3 = "null"
			} else {
				tmp3 = "not null"
			}
			return tmp3, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetNumberArray", unsetRes.Data.ApplyT(func(data optionalprimitiveref.Data) (string, error) {
			var tmp4 string
			if data.NumberArray == nil {
				tmp4 = "null"
			} else {
				tmp4 = "not null"
			}
			return tmp4, nil
		}).(pulumi.StringOutput))
		ctx.Export("unsetBooleanMap", unsetRes.Data.ApplyT(func(data optionalprimitiveref.Data) (string, error) {
			var tmp5 string
			if data.BooleanMap == nil {
				tmp5 = "null"
			} else {
				tmp5 = "not null"
			}
			return tmp5, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
