package main

import (
	"example.com/pulumi-extra-package-names/sdk/go/v47/extrapackagenames"
	"example.com/pulumi-extra-package-names/sdk/go/v47/extrapackagenames/mod"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := extrapackagenames.NewProvider(ctx, "prov", nil)
		if err != nil {
			return err
		}
		_, err = mod.NewRes(ctx, "viaProvider", &mod.ResArgs{
			Choice: mod.ChoiceFirst,
			Obj: &mod.ObjArgs{
				Label:  pulumi.String("explicit"),
				Choice: mod.ChoiceSecond,
			},
		}, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		_, err = mod.NewRes(ctx, "viaPackage", &mod.ResArgs{
			Choice: mod.ChoiceSecond,
			Obj: &mod.ObjArgs{
				Label:  pulumi.String("bare"),
				Choice: mod.ChoiceFirst,
			},
		})
		if err != nil {
			return err
		}
		thing := mod.GetThingOutput(ctx, mod.GetThingOutputArgs{
			Text: pulumi.String("hello"),
		})
		ctx.Export("result", thing.ApplyT(func(thing mod.GetThingResult) (string, error) {
			return thing.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
