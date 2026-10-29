package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		provider, err := simpleinvoke.NewProvider(ctx, "provider", nil)
		if err != nil {
			return err
		}
		parent, err := simpleinvoke.NewStringResource(ctx, "parent", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("parent"),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}
		// This should inherit the explicit provider from parent.
		child, err := simpleinvoke.NewStringResource(ctx, "child", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("child"),
		}, pulumi.Parent(parent))
		if err != nil {
			return err
		}
		// This should use the explicit provider of parent.
		parentData := simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("parent"),
		}, pulumi.Parent(parent))
		// This should use the explicit provider that child inherited from parent.
		childData := simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: pulumi.String("child"),
		}, pulumi.Parent(child))
		ctx.Export("parentResult", parentData.Result())
		ctx.Export("childResult", childData.Result())
		return nil
	})
}
