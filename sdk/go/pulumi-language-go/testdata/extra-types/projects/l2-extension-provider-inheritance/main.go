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
		// This should inherit prov from parent, because prov is the base provider of myext.
		_, err = myext.NewGreeting(ctx, "child", nil, pulumi.Parent(parent))
		if err != nil {
			return err
		}
		return nil
	})
}
