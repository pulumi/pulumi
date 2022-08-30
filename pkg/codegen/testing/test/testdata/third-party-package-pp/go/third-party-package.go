package main

import (
	"git.example.org/thirdparty"
	"git.example.org/thirdparty/module"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := thirdparty.NewThing(ctx, "Other", &thirdparty.ThingArgs{
			Idea: pulumi.String("Support Third Party"),
		})
		if err != nil {
			return err
		}
		_, err = module.NewObject(ctx, "Question", &module.ObjectArgs{
			Answer: pulumi.Float64(42),
		})
		if err != nil {
			return err
		}
		_, err = thirdparty.NewProvider(ctx, "Provider", &thirdparty.ProviderArgs{
			ObjectProp: pulumi.StringMap{
				"prop1": pulumi.String("foo"),
				"prop2": pulumi.String("bar"),
				"prop3": pulumi.String("fizz"),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
