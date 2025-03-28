package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		someComponent, err := NewMyComponent(ctx, "someComponent", &MyComponentArgs{
			Input: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("result", someComponent.Output)
		return nil
	})
}
