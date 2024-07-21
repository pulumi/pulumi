package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("output_true", pulumi.Bool(true))
		ctx.Export("output_false", pulumi.Bool(false))
		ctx.Export("output_number", pulumi.Float64(4))
		ctx.Export("output_string", pulumi.String("hello"))
		return nil
	})
}
