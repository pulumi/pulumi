package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("hello", simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("hello"),
		}, nil).ApplyT(func(invoke simpleinvoke.MyInvokeResult) (string, error) {
			return invoke.Result, nil
		}).(pulumi.StringOutput))
		ctx.Export("goodbye", simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("goodbye"),
		}, nil).ApplyT(func(invoke simpleinvoke.MyInvokeResult) (string, error) {
			return invoke.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
