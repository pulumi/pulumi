package main

import (
	"example.com/pulumi-myext/sdk/go/v2/myext"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		greeting, err := myext.NewGreeting(ctx, "greeting", nil)
		if err != nil {
			return err
		}
		greetingComp, err := myext.NewGreetingComponent(ctx, "greetingComp", nil)
		if err != nil {
			return err
		}
		ctx.Export("parameterValue", greeting.ParameterValue)
		ctx.Export("parameterValueFromComponent", greetingComp.ParameterValue)
		ctx.Export("invokeGreeting", myext.GreetOutput(ctx, myext.GreetOutputArgs{
			Name: pulumi.String("Pulumi"),
		}, nil).ApplyT(func(invoke myext.GreetResult) (string, error) {
			return invoke.Greeting, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
