package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("zero", pulumi.Float64(0))
		ctx.Export("one", pulumi.Float64(1))
		ctx.Export("e", pulumi.Float64(2.718))
		ctx.Export("minInt32", pulumi.Float64(-2147483648))
		ctx.Export("max", pulumi.Float64(1.7976931348623157e+308))
		ctx.Export("min", pulumi.Float64(5e-324))
		return nil
	})
}
