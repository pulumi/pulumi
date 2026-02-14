package main

import (
	"example.com/pulumi-module-format/sdk/go/v29/moduleformat/mod"
	"example.com/pulumi-module-format/sdk/go/v29/moduleformat/mod/nested"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// This tests that PCL allows both fully specified type tokens, and tokens that only specify the module and
		// member name.
		// First use the fully specified token to invoke and create a resource.
		res1, err := mod.NewResource(ctx, "res1", &mod.ResourceArgs{
			Text: mod.ConcatWorldOutput(ctx, mod.ConcatWorldOutputArgs{
				Value: pulumi.String("hello"),
			}, nil).ApplyT(func(invoke mod.ConcatWorldResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		out1, err := res1.Call(ctx, &mod.ResourceCallArgs{Input: pulumi.String("x")})
		if err != nil {
			return err
		}
		ctx.Export("out1", out1.Output())

		// Next use just the module name as defined by the module format
		res2, err := mod.NewResource(ctx, "res2", &mod.ResourceArgs{
			Text: mod.ConcatWorldOutput(ctx, mod.ConcatWorldOutputArgs{
				Value: pulumi.String("goodbye"),
			}, nil).ApplyT(func(invoke mod.ConcatWorldResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		out2, err := res2.Call(ctx, &mod.ResourceCallArgs{Input: pulumi.String("xx")})
		if err != nil {
			return err
		}
		ctx.Export("out2", out2.Output())

		// Now test nested module resources using the fully specified token.
		res3, err := nested.NewResource(ctx, "res3", &nested.ResourceArgs{
			Text: nested.ConcatWorldOutput(ctx, nested.ConcatWorldOutputArgs{
				Value: pulumi.String("hello"),
			}, nil).ApplyT(func(invoke nested.ConcatWorldResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		out3, err := res3.Call(ctx, &nested.ResourceCallArgs{Input: pulumi.String("x")})
		if err != nil {
			return err
		}
		ctx.Export("out3", out3.Output())

		// And using just the module name as defined by the module format.
		res4, err := nested.NewResource(ctx, "res4", &nested.ResourceArgs{
			Text: nested.ConcatWorldOutput(ctx, nested.ConcatWorldOutputArgs{
				Value: pulumi.String("goodbye"),
			}, nil).ApplyT(func(invoke nested.ConcatWorldResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		out4, err := res4.Call(ctx, &nested.ResourceCallArgs{Input: pulumi.String("xx")})
		if err != nil {
			return err
		}
		ctx.Export("out4", out4.Output())
		return nil
	})
}
