package main

import (
	"example.com/pulumi-subpackage/sdk/go/v2/subpackage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The resource name is based on the parameter value
		example, err := subpackage.NewHelloWorld(ctx, "example", nil)
		if err != nil {
			return err
		}
		exampleComponent, err := subpackage.NewHelloWorldComponent(ctx, "exampleComponent", nil)
		if err != nil {
			return err
		}
		ctx.Export("parameterValue", example.ParameterValue)
		ctx.Export("parameterValueFromComponent", exampleComponent.ParameterValue)
		return nil
	})
}
