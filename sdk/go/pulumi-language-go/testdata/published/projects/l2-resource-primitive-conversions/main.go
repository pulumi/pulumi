package main

import (
	"strconv"

	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		plainBool := cfg.RequireBool("plainBool")
		plainNumber := cfg.RequireFloat64("plainNumber")
		plainInteger := cfg.RequireInt("plainInteger")
		plainString := cfg.Require("plainString")
		plainNumericString := cfg.Require("plainNumericString")
		secretNumber := cfg.RequireSecretFloat64("secretNumber")
		secretInteger := cfg.RequireSecretInt("secretInteger")
		secretString := cfg.RequireSecret("secretString")
		secretNumericString := cfg.RequireSecret("secretNumericString")
		_, err := primitive.NewResource(ctx, "plainValues", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(plainString == "true"),
			Float:   pulumi.Float64(float64(plainInteger)),
			Integer: pulumi.Int(func() int {
				i, err := strconv.Atoi(plainNumericString)
				if err != nil {
					panic(err)
				}
				return i
			}()),
			String: pulumi.String(strconv.FormatFloat(plainNumber, 'f', -1, 64)),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(float64(plainInteger)),
				pulumi.Float64(func() float64 {
					f, err := strconv.ParseFloat(plainNumericString, 64)
					if err != nil {
						panic(err)
					}
					return f
				}()),
				pulumi.Float64(plainNumber),
			},
			BooleanMap: pulumi.BoolMap{
				"fromBool":   pulumi.Bool(plainBool),
				"fromString": pulumi.Bool(plainString == "true"),
			},
		})
		if err != nil {
			return err
		}
		_, err = primitive.NewResource(ctx, "secretValues", &primitive.ResourceArgs{
			Boolean: secretString.ApplyT(func(v string) bool { return v == "true" }).(pulumi.BoolOutput),
			Float:   secretInteger.ApplyT(func(v int) float64 { return float64(v) }).(pulumi.Float64Output),
			Integer: secretNumericString.ApplyT(func(v string) int {
				return func() int {
					i, err := strconv.Atoi(v)
					if err != nil {
						panic(err)
					}
					return i
				}()
			}).(pulumi.IntOutput),
			String: secretNumber.ApplyT(func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }).(pulumi.StringOutput),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(float64(plainInteger)),
				pulumi.Float64(func() float64 {
					f, err := strconv.ParseFloat(plainNumericString, 64)
					if err != nil {
						panic(err)
					}
					return f
				}()),
				pulumi.Float64(plainNumber),
			},
			BooleanMap: pulumi.BoolMap{
				"fromBool":   pulumi.Bool(plainBool),
				"fromString": pulumi.Bool(plainString == "true"),
			},
		})
		if err != nil {
			return err
		}
		invokeResult := primitive.InvokeOutput(ctx, primitive.InvokeOutputArgs{
			Boolean: pulumi.Bool(plainString == "true"),
			Float:   pulumi.Float64(float64(plainInteger)),
			Integer: pulumi.Int(func() int {
				i, err := strconv.Atoi(plainNumericString)
				if err != nil {
					panic(err)
				}
				return i
			}()),
			String: pulumi.String(strconv.FormatBool(plainBool)),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(float64(plainInteger)),
				pulumi.Float64(func() float64 {
					f, err := strconv.ParseFloat(plainNumericString, 64)
					if err != nil {
						panic(err)
					}
					return f
				}()),
				pulumi.Float64(plainNumber),
			},
			BooleanMap: pulumi.BoolMap{
				"fromBool":   pulumi.Bool(plainBool),
				"fromString": pulumi.Bool(plainString == "true"),
			},
		}, nil)
		_, err = primitive.NewResource(ctx, "invokeValues", &primitive.ResourceArgs{
			Boolean: invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) (bool, error) {
				return invokeResult.Boolean, nil
			}).(pulumi.BoolOutput),
			Float: invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) (float64, error) {
				return invokeResult.Float, nil
			}).(pulumi.Float64Output),
			Integer: invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) (int, error) {
				return invokeResult.Integer, nil
			}).(pulumi.IntOutput),
			String: invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) (string, error) {
				return invokeResult.String, nil
			}).(pulumi.StringOutput),
			NumberArray: invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) ([]float64, error) {
				return invokeResult.NumberArray, nil
			}).(pulumi.Float64ArrayOutput),
			BooleanMap: invokeResult.ApplyT(func(invokeResult primitive.InvokeResult) (map[string]bool, error) {
				return invokeResult.BooleanMap, nil
			}).(pulumi.BoolMapOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
