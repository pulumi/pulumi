package main

import (
	"example.com/pulumi-scalar-returns/sdk/go/v21/scalarreturns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("secret", scalarreturns.InvokeSecretOutput(ctx, scalarreturns.InvokeSecretOutputArgs{
			Value: pulumi.String("goodbye"),
		}, nil))
		ctx.Export("array", scalarreturns.InvokeArrayOutput(ctx, scalarreturns.InvokeArrayOutputArgs{
			Value: pulumi.String("the word"),
		}, nil))
		ctx.Export("map", scalarreturns.InvokeMapOutput(ctx, scalarreturns.InvokeMapOutputArgs{
			Value: pulumi.String("hello"),
		}, nil))
		ctx.Export("secretMap", scalarreturns.InvokeMapOutput(ctx, scalarreturns.InvokeMapOutputArgs{
			Value: pulumi.String("secret"),
		}, nil))
		return nil
	})
}
