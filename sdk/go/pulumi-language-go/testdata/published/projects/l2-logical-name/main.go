package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		configLexicalName := cfg.RequireBool("cC-Charlie_charlie.😃⁉️")
		resourceLexicalName, err := simple.NewResource(ctx, "aA-Alpha_alpha.🤯⁉️", &simple.ResourceArgs{
			Value: pulumi.Bool(configLexicalName),
		})
		if err != nil {
			return err
		}
		ctx.Export("bB-Beta_beta.💜⁉", resourceLexicalName.Value)
		ctx.Export("dD-Delta_delta.🔥⁉", resourceLexicalName.Value)
		return nil
	})
}
