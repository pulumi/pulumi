package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		explicitProvider, err := simpleinvoke.NewProvider(ctx, "explicitProvider", nil)
		if err != nil {
			return err
		}
		data := simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("hello"),
		}, pulumi.Provider(explicitProvider), pulumi.Parent(explicitProvider), pulumi.Version("10.0.0"), pulumi.PluginDownloadURL("https://example.com/github/example"))
		ctx.Export("hello", data.ApplyT(func(data simpleinvoke.MyInvokeResult) (string, error) {
			return data.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
