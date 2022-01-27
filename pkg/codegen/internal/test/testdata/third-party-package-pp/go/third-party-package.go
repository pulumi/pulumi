package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gitplace.org/thirdparty"
	"gitplace.org/thirdparty/module"
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
		return nil
	})
}
