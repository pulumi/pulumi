package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		aSecret := cfg.RequireSecret("aSecret")
		notSecret := cfg.Require("notSecret")
		ctx.Export("roundtripSecret", aSecret)
		ctx.Export("roundtripNotSecret", pulumi.String(notSecret))
		ctx.Export("double", pulumi.ToSecret(aSecret).(pulumi.StringOutput))
		ctx.Export("open", pulumi.Unsecret(aSecret).(pulumi.StringOutput))
		ctx.Export("close", pulumi.ToSecret(notSecret).(pulumi.StringOutput))
		return nil
	})
}
