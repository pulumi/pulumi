package main

import (
	"crypto/sha1"
	"encoding/hex"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func sha1Hash(input string) string {
	hash := sha1.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		input := cfg.Require("input")
		hash := sha1Hash(input)
		ctx.Export("hash", pulumi.String(hash))
		return nil
	})
}
