package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("output_true", true)
		ctx.Export("output_false", false)
		ctx.Export("output_number", 4)
		ctx.Export("output_string", "hello")
		return nil
	})
}
