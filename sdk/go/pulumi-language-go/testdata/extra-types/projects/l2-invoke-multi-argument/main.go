package main

import (
	"example.com/pulumi-multi-argument-invoke/sdk/go/v44/multiargumentinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("both", multiargumentinvoke.MultiArgumentInvokeOutput(ctx, pulumi.String("hello"), pulumi.String("world")).ApplyT(func(invoke multiargumentinvoke.MultiArgumentInvokeResult) (string, error) {
			return invoke.Result, nil
		}).(pulumi.StringOutput))
		ctx.Export("onlyRequired", multiargumentinvoke.MultiArgumentInvokeOutput(ctx, pulumi.String("hello"), nil).ApplyT(func(invoke multiargumentinvoke.MultiArgumentInvokeResult) (string, error) {
			return invoke.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
