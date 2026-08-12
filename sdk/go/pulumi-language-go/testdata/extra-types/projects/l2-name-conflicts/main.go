package main

import (
	moduleformatmod "example.com/pulumi-module-format/sdk/go/v29/moduleformat/mod"
	"example.com/pulumi-names/sdk/go/v6/names/mod"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		names := true
		if param := cfg.GetBool("names"); param {
			names = param
		}
		Names := true
		if param := cfg.GetBool("Names"); param {
			Names = param
		}
		mod2 := "module"
		if param := cfg.Get("mod"); param != "" {
			mod2 = param
		}
		Mod := "format"
		if param := cfg.Get("Mod"); param != "" {
			Mod = param
		}
		namesResource, err := mod.NewRes(ctx, "namesResource", &mod.ResArgs{
			Value: pulumi.Bool(names),
		})
		if err != nil {
			return err
		}
		modResource, err := moduleformatmod.NewResource(ctx, "modResource", &moduleformatmod.ResourceArgs{
			Text: pulumi.Sprintf("%v-%v", mod2, Mod),
		})
		if err != nil {
			return err
		}
		ctx.Export("namesResourceVal", namesResource.Value)
		ctx.Export("modResourceText", modResource.Text)
		ctx.Export("nameVariables", pulumi.Bool(names && Names))
		ctx.Export("modVariables", pulumi.Sprintf("%v-%v", mod2, Mod))
		return nil
	})
}
