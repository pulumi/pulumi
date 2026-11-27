package main

import (
	"example.com/pulumi-lookup-invoke/sdk/go/v63/lookupinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		widget, err := lookupinvoke.NewWidget(ctx, "widget", &lookupinvoke.WidgetArgs{
			Name: pulumi.String("resource"),
		})
		if err != nil {
			return err
		}
		// getWidget is a root-module function whose name clashes with the Widget
		// resource's getter in some SDKs (the Go SDK exports it as LookupWidget).
		fromLiteral := lookupinvoke.LookupWidgetOutput(ctx, lookupinvoke.LookupWidgetOutputArgs{
			Name: pulumi.String("literal"),
		}, nil)
		fromResource := lookupinvoke.LookupWidgetOutput(ctx, lookupinvoke.LookupWidgetOutputArgs{
			Name: widget.Name,
		}, nil)
		ctx.Export("fromLiteral", fromLiteral.Result())
		ctx.Export("fromResource", fromResource.Result())
		return nil
	})
}
