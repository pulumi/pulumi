package main

import (
	"example.com/pulumi-output/sdk/go/v23/output"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := output.NewProvider(ctx, "prov", &output.ProviderArgs{
			ElideUnknowns: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		provNotElided, err := output.NewProvider(ctx, "provNotElided", nil)
		if err != nil {
			return err
		}
		topLevel, err := output.NewResource(ctx, "topLevel", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		}, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		topLevelNotElided, err := output.NewResource(ctx, "topLevelNotElided", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		}, pulumi.Provider(provNotElided))
		if err != nil {
			return err
		}
		ctx.Export("topLevel", topLevel.SecretOutput)
		ctx.Export("topLevelNotElided", topLevelNotElided.SecretOutput)
		return nil
	})
}
