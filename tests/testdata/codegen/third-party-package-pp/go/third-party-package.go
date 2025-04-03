package main

import (
	other "git.example.org/thirdparty/sdk/go/pkg"
	"git.example.org/thirdparty/sdk/go/pkg/module"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := other.NewThing(ctx, "Other", &other.ThingArgs{
			Idea: "Support Third Party",
		})
		if err != nil {
			return err
		}
		_, err = module.NewObject(ctx, "Question", &module.ObjectArgs{
			Answer: 42,
		})
		if err != nil {
			return err
		}
		_, err = module.NewObject(ctx, "Question2", &module.ObjectArgs{
			Answer: 24,
		})
		if err != nil {
			return err
		}
		_, err = other.NewProvider(ctx, "Provider", &other.ProviderArgs{
			ObjectProp: map[string]pulumi.String{
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
