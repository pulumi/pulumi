package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		input, err := simple.NewResource(ctx, "input", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		someComponent, err := NewMyComponent(ctx, "someComponent", &MyComponentArgs{
			Input: input.Value,
		})
		if err != nil {
			return err
		}
		ctx.Export("result", someComponent.Output)
		return nil
	})
}
