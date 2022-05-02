package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		resourceLexicalName, err := random.NewRandomPet(ctx, "aA-Alpha_alpha.🤯⁉️", nil)
		if err != nil {
			return err
		}
		ctx.Export("bB-Beta_beta.💜⁉", resourceLexicalName.ID())
		return nil
	})
}
