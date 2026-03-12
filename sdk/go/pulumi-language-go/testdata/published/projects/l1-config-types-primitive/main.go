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
		optionalNumber := float64(41)
		if param := cfg.GetFloat64("optionalNumber"); param != 0 {
			optionalNumber = param
		}
		ctx.Export("defaultNumber", pulumi.Float64(optionalNumber+1))
		aString := cfg.Require("aString")
		ctx.Export("theString", pulumi.Sprintf("%v World", aString))
		optionalString := "defaultStringValue"
		if param := cfg.Get("optionalString"); param != "" {
			optionalString = param
		}
		ctx.Export("defaultString", pulumi.String(optionalString))
		aBool := cfg.RequireBool("aBool")
		ctx.Export("theBool", pulumi.Bool(!aBool && true))
		optionalBool := false
		if param := cfg.GetBool("optionalBool"); param {
			optionalBool = param
		}
		ctx.Export("defaultBool", pulumi.Bool(optionalBool))
		return nil
	})
}
