package main

import (
	"encoding/base64"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		input := cfg.Require("input")
		tmpVar0, _ := base64.StdEncoding.DecodeString(input)
		bytes := string(tmpVar0)
		ctx.Export("data", pulumi.String(bytes))
		ctx.Export("roundtrip", pulumi.String(base64.StdEncoding.EncodeToString([]byte(bytes))))
		return nil
	})
}
