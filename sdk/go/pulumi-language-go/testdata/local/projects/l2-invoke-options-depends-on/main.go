package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simpleinvoke.NewProvider(ctx, "explicitProvider", nil)
		if err != nil {
			return err
		}
		first, err := simpleinvoke.NewStringResource(ctx, "first", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("first hello"),
		})
		if err != nil {
			return err
		}
		data := simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("hello"),
		}, pulumi.DependsOn([]pulumi.Resource{
			pulumi.Resource(first),
		}))
		_, err = simpleinvoke.NewStringResource(ctx, "second", &simpleinvoke.StringResourceArgs{
			Text: data.ApplyT(func(data simpleinvoke.MyInvokeResult) (string, error) {
				return data.Result, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		ctx.Export("hello", data.ApplyT(func(data simpleinvoke.MyInvokeResult) (string, error) {
			return data.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
