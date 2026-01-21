package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		version := cfg.Require("version")
		if err := ctx.CheckPulumiVersion(version); err != nil {
			return err
		}
		return nil
	})
}
