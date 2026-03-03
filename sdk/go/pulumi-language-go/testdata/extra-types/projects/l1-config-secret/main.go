package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		aNumber := cfg.RequireSecretFloat64("aNumber")
		ctx.Export("theSecretNumber", aNumber.ApplyT(func(aNumber float64) (float64, error) {
			return aNumber + 1.25, nil
		}).(pulumi.Float64Output))
		return nil
	})
}
