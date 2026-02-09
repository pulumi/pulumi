package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		aNumber := cfg.RequireFloat64("aNumber")
		ctx.Export("theNumber", pulumi.Float64(aNumber+1.25))
		aString := cfg.Require("aString")
		ctx.Export("theString", pulumi.Sprintf("%v World", aString))
		aBool := cfg.RequireBool("aBool")
		ctx.Export("theBool", pulumi.Bool(!aBool && true))
		return nil
	})
}
