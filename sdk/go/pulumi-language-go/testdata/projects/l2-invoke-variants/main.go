package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := simpleinvoke.NewStringResource(ctx, "res", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("hello"),
		})
		if err != nil {
			return err
		}
		ctx.Export("outputInput", simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: res.Text,
		}, nil).ApplyT(func(invoke simpleinvoke.MyInvokeResult) (string, error) {
			return invoke.Result, nil
		}).(pulumi.StringOutput))
		ctx.Export("unit", simpleinvoke.UnitOutput(ctx, simpleinvoke.UnitOutputArgs{}, nil).ApplyT(func(invoke simpleinvoke.UnitResult) (string, error) {
			return invoke.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
