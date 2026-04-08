package main

import (
	"example.com/pulumi-index-mod/sdk/go/v35/indexmod/indexmine"
	"example.com/pulumi-index-mod/sdk/go/v35/indexmod/indexmine/nested"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res1, err := indexmine.NewResource(ctx, "res1", &indexmine.ResourceArgs{
			Text: indexmine.ConcatWorldOutput(ctx, indexmine.ConcatWorldOutputArgs{
				Value: pulumi.String("hello"),
			}, nil).ApplyT(func(invoke indexmine.ConcatWorldResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		callCall, err := res1.Call(ctx, &indexmine.ResourceCallArgs{
			Input: pulumi.String("x"),
		})
		if err != nil {
			return err
		}
		ctx.Export("out1", callCall.ApplyT(func(call indexmine.ResourceCallResult) (float64, error) {
			return call.Output, nil
		}).(pulumi.Float64Output))
		res2, err := nested.NewResource(ctx, "res2", &nested.ResourceArgs{
			Text: nested.ConcatWorldOutput(ctx, nested.ConcatWorldOutputArgs{
				Value: pulumi.String("goodbye"),
			}, nil).ApplyT(func(invoke nested.ConcatWorldResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		callCall1, err := res2.Call(ctx, &nested.ResourceCallArgs{
			Input: pulumi.String("xx"),
		})
		if err != nil {
			return err
		}
		ctx.Export("out2", callCall1.ApplyT(func(call nested.ResourceCallResult) (float64, error) {
			return call.Output, nil
		}).(pulumi.Float64Output))
		return nil
	})
}
