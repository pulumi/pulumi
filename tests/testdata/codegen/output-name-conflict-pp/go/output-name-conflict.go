package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		cidrBlock := "Test config variable"
		if param := cfg.Get("cidrBlock"); param != "" {
			cidrBlock = param
		}
		ctx.Export("cidrBlock", cidrBlock)
		return nil
	})
}
