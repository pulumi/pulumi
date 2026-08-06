package main

import (
	"example.com/pulumi-discriminated-union-internal/sdk/go/v50/discriminatedunioninternal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := discriminatedunioninternal.NewExample(ctx, "example1", &discriminatedunioninternal.ExampleArgs{
			UnionOf: &discriminatedunioninternal.AlphaArgs{
				Type__:  pulumi.String("Alpha"),
				Payload: pulumi.String("p1"),
				Weight:  pulumi.Int(1),
			},
			SecretUnion: &discriminatedunioninternal.BetaArgs{
				Type__:  pulumi.String("Beta"),
				Payload: pulumi.String("s1"),
				Tint:    pulumi.String("blue"),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunioninternal.NewExample(ctx, "example2", &discriminatedunioninternal.ExampleArgs{
			UnionOf: &discriminatedunioninternal.BetaArgs{
				Type__:  pulumi.String("Beta"),
				Payload: pulumi.String("p2"),
				Tint:    pulumi.String("red"),
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunioninternal.NewExample(ctx, "example3", &discriminatedunioninternal.ExampleArgs{
			UnionOf: &discriminatedunioninternal.GammaArgs{
				Type__:  pulumi.String("Gamma"),
				Payload: pulumi.String("p3"),
				Active:  pulumi.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
