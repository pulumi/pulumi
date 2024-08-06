package main

import (
	"github.com/pulumi/pulumi-std/sdk/go/std"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_ = std.ReplaceOutput(ctx, std.ReplaceOutputArgs{
			Text: std.UpperOutput(ctx, std.UpperOutputArgs{
				Input: pulumi.String("hello_world"),
			}, nil).ApplyT(func(invoke std.UpperResult) (string, error) {
				return invoke.Result, nil
			}).(pulumi.StringOutput),
			Search:  pulumi.String("_"),
			Replace: pulumi.String("-"),
		}, nil)
		return nil
	})
}
