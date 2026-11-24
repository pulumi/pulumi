package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("result", simpleinvoke.InvokeWithDefaultOutput(ctx, simpleinvoke.InvokeWithDefaultOutputArgs{}, nil).Result())
		ctx.Export("explicitResult", simpleinvoke.InvokeWithDefaultOutput(ctx, simpleinvoke.InvokeWithDefaultOutputArgs{
			Value: pulumi.String("explicit"),
		}, nil).Result())
		return nil
	})
}
