package main

import (
	"example.com/pulumi-output/sdk/go/v23/output"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// This test checks that when a provider doesn't return properties for fields it considers unknown the runtime
		// can still access that field as an output.
		prov, err := output.NewProvider(ctx, "prov", &output.ProviderArgs{
			ElideUnknowns: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		unknown, err := output.NewResource(ctx, "unknown", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		}, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		// Try and use the unknown output as an input to another resource to check that it doesn't cause any issues.
		_, err = simple.NewResource(ctx, "res", &simple.ResourceArgs{
			Value: unknown.Output.ApplyT(func(output string) (bool, error) {
				return output == "hello", nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		ctx.Export("out", unknown.Output)
		return nil
	})
}
