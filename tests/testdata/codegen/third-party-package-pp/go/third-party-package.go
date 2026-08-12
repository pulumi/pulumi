package main

import (
	other "git.example.org/thirdparty/sdk/go/pkg"
	"git.example.org/thirdparty/sdk/go/pkg/module"
	"git.example.org/thirdparty/sdk/go/pkg/module/sub"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := other.NewThing(ctx, "Other", &other.ThingArgs{
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
		_, err = sub.NewObject(ctx, "Question2", &sub.ObjectArgs{
			Answer: pulumi.Float64(24),
		})
		if err != nil {
			return err
		}
		_, err = other.NewProvider(ctx, "Provider", &other.ProviderArgs{
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
