package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		configLexicalName := cfg.Require("cC-Charlie_charlie.😃⁉️")
		resourceLexicalName, err := random.NewRandomPet(ctx, "aA-Alpha_alpha.🤯⁉️", &random.RandomPetArgs{
			Prefix: pulumi.String(configLexicalName),
		})
		if err != nil {
			return err
		}
		ctx.Export("bB-Beta_beta.💜⁉", resourceLexicalName.ID())
		ctx.Export("dD-Delta_delta.🔥⁉", resourceLexicalName.ID())
		return nil
	})
}
