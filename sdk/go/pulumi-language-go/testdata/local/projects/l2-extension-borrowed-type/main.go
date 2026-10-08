package main

import (
	"example.com/pulumi-myborrow/sdk/go/v3/myborrow"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		widget, err := myborrow.NewWidget(ctx, "widget", nil)
		if err != nil {
			return err
		}
		ctx.Export("metadataName", widget.Metadata.Name())
		return nil
	})
}
