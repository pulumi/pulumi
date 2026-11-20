package main

import (
	"example.com/pulumi-failingcomponent/sdk/go/v59/failingcomponent"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := failingcomponent.NewComponent(ctx, "component", nil)
		if err != nil {
			return err
		}
		return nil
	})
}
