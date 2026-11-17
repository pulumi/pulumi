package main

import (
	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		plainBool := cfg.RequireBool("plainBool")
		plainNumber := cfg.RequireFloat64("plainNumber")
		plainInteger := cfg.RequireInt("plainInteger")
		plainString := cfg.Require("plainString")
		secretBool := cfg.RequireSecretBool("secretBool")
		secretNumber := cfg.RequireSecretFloat64("secretNumber")
		secretInteger := cfg.RequireSecretInt("secretInteger")
		secretString := cfg.RequireSecret("secretString")
		_, err := primitive.NewResource(ctx, "plain", &primitive.ResourceArgs{
			Boolean: pulumi.Bool(plainBool),
			Float:   pulumi.Float64(plainNumber),
			Integer: pulumi.Int(plainInteger),
			String:  pulumi.String(plainString),
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(-1),
				pulumi.Float64(0),
				pulumi.Float64(1),
			},
			BooleanMap: pulumi.BoolMap{
				"t": pulumi.Bool(true),
				"f": pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		_, err = primitive.NewResource(ctx, "secret", &primitive.ResourceArgs{
			Boolean: secretBool,
			Float:   secretNumber,
			Integer: secretInteger,
			String:  secretString,
			NumberArray: pulumi.Float64Array{
				pulumi.Float64(-2),
				pulumi.Float64(0),
				pulumi.Float64(2),
			},
			BooleanMap: pulumi.BoolMap{
				"t": pulumi.Bool(true),
				"f": pulumi.Bool(false),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
