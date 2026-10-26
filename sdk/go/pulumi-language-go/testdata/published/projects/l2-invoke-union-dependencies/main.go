package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Baseline for invoke dependency propagation: an invoke that reads properties from two different
		// resources produces a return value whose consumer must depend on the union of both.
		a, err := simpleinvoke.NewStringResource(ctx, "a", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("hello"),
		})
		if err != nil {
			return err
		}
		b, err := simple.NewResource(ctx, "b", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		data := simpleinvoke.SecretInvokeOutput(ctx, simpleinvoke.SecretInvokeOutputArgs{
			Value:          a.Text,
			SecretResponse: b.Value,
		}, nil)
		_, err = simpleinvoke.NewStringResource(ctx, "d", &simpleinvoke.StringResourceArgs{
			Text: data.Response(),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
