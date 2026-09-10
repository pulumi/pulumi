package main

import (
	"example.com/pulumi-output/sdk/go/v23/output"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		provElided, err := output.NewProvider(ctx, "provElided", &output.ProviderArgs{
			ElideUnknowns: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		provNotElided, err := output.NewProvider(ctx, "provNotElided", nil)
		if err != nil {
			return err
		}
		topLevelElided, err := output.NewResource(ctx, "topLevelElided", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		}, pulumi.Provider(provElided))
		if err != nil {
			return err
		}
		topLevelNotElided, err := output.NewResource(ctx, "topLevelNotElided", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		}, pulumi.Provider(provNotElided))
		if err != nil {
			return err
		}
		ctx.Export("topLevelElided", topLevelElided.SecretOutput)
		ctx.Export("topLevelNotElided", topLevelNotElided.SecretOutput)
		return nil
	})
}
