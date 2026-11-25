package main

import (
	"example.com/pulumi-moduleformatenum/sdk/go/v64/moduleformatenum"
	"example.com/pulumi-moduleformatenum/sdk/go/v64/moduleformatenum/mod"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Enum literals for enum types whose modules have to be resolved with the module format.
		_, err := mod.NewRes(ctx, "res", &mod.ResArgs{
			Color: mod.ColorBlue,
			Shade: moduleformatenum.ShadeDark,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
