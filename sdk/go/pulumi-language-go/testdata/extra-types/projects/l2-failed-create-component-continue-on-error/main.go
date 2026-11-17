package main

import (
	"example.com/pulumi-failingcomponent/sdk/go/v59/failingcomponent"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		comp, err := failingcomponent.NewComponent(ctx, "comp", nil)
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "dependent", &simple.ResourceArgs{
			Value: comp.Value.ApplyT(func(value string) (bool, error) {
				return value == "hello", nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "independent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("componentValue", comp.Value)
		return nil
	})
}
