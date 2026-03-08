package main

import (
	"example.com/pulumi-subpackage/sdk/go/v2/subpackage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("parameterValue", subpackage.DoHelloWorldOutput(ctx, subpackage.DoHelloWorldOutputArgs{
			Input: pulumi.String("goodbye"),
		}, nil).ApplyT(func(invoke subpackage.DoHelloWorldResult) (string, error) {
			return invoke.Output, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
