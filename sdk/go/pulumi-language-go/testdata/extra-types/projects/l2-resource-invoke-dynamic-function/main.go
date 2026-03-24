package main

import (
	"example.com/pulumi-any-type-function/sdk/go/v15/anytypefunction"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		localValue := "hello"
		ctx.Export("dynamic", anytypefunction.DynListToDynOutput(ctx, anytypefunction.DynListToDynOutputArgs{
			Inputs: pulumi.Array{
				pulumi.Any("hello"),
				pulumi.String(localValue),
				pulumi.Any(map[string]interface{}{}),
			},
		}, nil).ApplyT(func(invoke anytypefunction.DynListToDynResult) (interface{}, error) {
			return invoke.Result, nil
		}).(pulumi.AnyOutput))
		return nil
	})
}
