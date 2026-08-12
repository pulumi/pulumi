package main

import (
	"example.com/pulumi-output-only-invoke/sdk/go/v24/outputonlyinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("hello", outputonlyinvoke.MyInvokeOutput(ctx, outputonlyinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("hello"),
		}, nil).Result())
		ctx.Export("goodbye", outputonlyinvoke.MyInvokeOutput(ctx, outputonlyinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("goodbye"),
		}, nil).Result())
		return nil
	})
}
