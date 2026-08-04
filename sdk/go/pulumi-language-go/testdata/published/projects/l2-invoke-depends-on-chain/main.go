package main

import (
	"example.com/pulumi-simple-invoke/sdk/go/v10/simpleinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		first, err := simpleinvoke.NewStringResource(ctx, "first", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("first"),
		})
		if err != nil {
			return err
		}
		second, err := simpleinvoke.NewStringResource(ctx, "second", &simpleinvoke.StringResourceArgs{
			Text: pulumi.String("second"),
		})
		if err != nil {
			return err
		}
		// getText fails unless a StringResource has already been created, so an SDK
		// that drops the dependsOn option calls it during preview and fails the test.
		gated := simpleinvoke.GetTextOutput(ctx, simpleinvoke.GetTextOutputArgs{
			Text: pulumi.String("Goodbye"),
		}, pulumi.DependsOn([]pulumi.Resource{
			pulumi.Resource(first),
		}))
		// myInvoke fails when called with an unknown argument, so an SDK that does not
		// await the gated invoke before chaining calls it during preview and fails the
		// test.
		chained := simpleinvoke.MyInvokeOutput(ctx, simpleinvoke.MyInvokeOutputArgs{
			Value: gated.Result(),
		}, pulumi.DependsOn([]pulumi.Resource{
			pulumi.Resource(second),
		}))
		ctx.Export("result", chained.Result())
		return nil
	})
}
