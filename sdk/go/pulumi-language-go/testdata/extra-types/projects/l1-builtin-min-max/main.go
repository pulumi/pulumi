package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		a := cfg.RequireFloat64("a")
		b := cfg.RequireFloat64("b")
		c := cfg.RequireInt("c")
		d := cfg.RequireInt("d")
		ctx.Export("maxResult", pulumi.Float64(max(a, b)))
		ctx.Export("minResult", pulumi.Float64(min(a, b)))
		ctx.Export("intMaxResult", pulumi.Int(max(c, d)))
		ctx.Export("intMinResult", pulumi.Int(min(c, d)))
		return nil
	})
}
