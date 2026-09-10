package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := simple.NewResource(ctx, "res", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("nonSecret", simpleinvoke.SecretInvokeOutput(ctx, simpleinvoke.SecretInvokeOutputArgs{
			Value:          pulumi.String("hello"),
			SecretResponse: pulumi.Bool(false),
		}, nil).Response())
		ctx.Export("firstSecret", simpleinvoke.SecretInvokeOutput(ctx, simpleinvoke.SecretInvokeOutputArgs{
			Value:          pulumi.String("hello"),
			SecretResponse: res.Value,
		}, nil).Response())
		ctx.Export("secondSecret", simpleinvoke.SecretInvokeOutput(ctx, simpleinvoke.SecretInvokeOutputArgs{
			Value:          pulumi.ToSecret("goodbye").(pulumi.StringOutput),
			SecretResponse: pulumi.Bool(false),
		}, nil).Response())
		return nil
	})
}
