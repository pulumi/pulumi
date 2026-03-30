package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		plainBool := cfg.RequireBool("plainBool")
		plainNumber := cfg.RequireFloat64("plainNumber")
		plainString := cfg.Require("plainString")
		secretBool := cfg.RequireSecretBool("secretBool")
		secretNumber := cfg.RequireSecretFloat64("secretNumber")
		secretString := cfg.RequireSecret("secretString")
		_, err := NewPrimitiveComponent(ctx, "plain", &PrimitiveComponentArgs{
			Boolean: pulumi.Bool(plainBool),
			Float:   pulumi.Float64(plainNumber + 0.5),
			Integer: pulumi.Int(int(plainNumber)),
			String:  pulumi.String(plainString),
		})
		if err != nil {
			return err
		}
		_, err = NewPrimitiveComponent(ctx, "secret", &PrimitiveComponentArgs{
			Boolean: secretBool,
			Float: secretNumber.ApplyT(func(secretNumber float64) (float64, error) {
				return secretNumber + 0.5, nil
			}).(pulumi.Float64Output),
			Integer: secretNumber.ApplyT(func(v float64) int { return int(v) }).(pulumi.IntOutput),
			String:  secretString,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
