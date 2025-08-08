package main

import (
	"example.com/pulumi-byepackage/sdk/go/v2/byepackage"
	"example.com/pulumi-hipackage/sdk/go/v2/hipackage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The resource name is based on the parameter value
		example1, err := hipackage.NewHelloWorld(ctx, "example1", nil)
		if err != nil {
			return err
		}
		exampleComponent1, err := hipackage.NewHelloWorldComponent(ctx, "exampleComponent1", nil)
		if err != nil {
			return err
		}
		ctx.Export("parameterValue1", example1.ParameterValue)
		ctx.Export("parameterValueFromComponent1", exampleComponent1.ParameterValue)
		// The resource name is based on the parameter value
		example2, err := byepackage.NewGoodbyeWorld(ctx, "example2", nil)
		if err != nil {
			return err
		}
		exampleComponent2, err := byepackage.NewGoodbyeWorldComponent(ctx, "exampleComponent2", nil)
		if err != nil {
			return err
		}
		ctx.Export("parameterValue2", example2.ParameterValue)
		ctx.Export("parameterValueFromComponent2", exampleComponent2.ParameterValue)
		return nil
	})
}
