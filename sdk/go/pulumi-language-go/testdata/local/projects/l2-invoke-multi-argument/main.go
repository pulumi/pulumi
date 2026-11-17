package main

import (
	"example.com/pulumi-multi-argument-invoke/sdk/go/v44/multiargumentinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("both", multiargumentinvoke.MultiArgumentInvokeOutput(ctx, pulumi.String("hello"), pulumi.String("world")).Result())
		ctx.Export("onlyRequired", multiargumentinvoke.MultiArgumentInvokeOutput(ctx, pulumi.String("hello"), nil).Result())
		return nil
	})
}
