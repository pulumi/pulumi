package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		first, err := simple.NewResource(ctx, "first", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		// assert that resource second depends on resource first
		// because it uses .secret from the invoke which depends on first
		_, err = simple.NewResource(ctx, "second", &simple.ResourceArgs{
			Value: simpleinvoke.SecretInvokeOutput(ctx, simpleinvoke.SecretInvokeOutputArgs{
				Value:          pulumi.String("hello"),
				SecretResponse: first.Value,
			}, nil).ApplyT(func(invoke simpleinvoke.SecretInvokeResult) (bool, error) {
				return invoke.Secret, nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
