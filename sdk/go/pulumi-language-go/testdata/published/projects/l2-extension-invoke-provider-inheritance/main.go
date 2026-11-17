package main

import (
	"example.com/pulumi-extbase/sdk/go/v45/extbase"
	"example.com/pulumi-myext/sdk/go/v2/myext"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := extbase.NewProvider(ctx, "prov", nil)
		if err != nil {
			return err
		}
		parent, err := extbase.NewBase(ctx, "parent", nil, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		// This should use prov from parent, because prov is the base provider of myext.
		greeting := myext.GreetOutput(ctx, myext.GreetOutputArgs{
			Name: pulumi.String("World"),
		}, pulumi.Parent(parent))
		ctx.Export("greeting", greeting.Greeting())
		return nil
	})
}
