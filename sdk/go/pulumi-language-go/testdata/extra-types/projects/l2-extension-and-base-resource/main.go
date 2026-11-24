package main

import (
	"example.com/pulumi-extbase/sdk/go/v45/extbase"
	"example.com/pulumi-myext/sdk/go/v2/myext"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		greeting, err := myext.NewGreeting(ctx, "greeting", nil)
		if err != nil {
			return err
		}
		base, err := extbase.NewBase(ctx, "base", nil)
		if err != nil {
			return err
		}
		ctx.Export("parameterValue", greeting.ParameterValue)
		ctx.Export("baseValue", base.BaseValue)
		return nil
	})
}
