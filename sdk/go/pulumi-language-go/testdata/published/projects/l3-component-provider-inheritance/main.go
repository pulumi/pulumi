package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		explicit, err := simple.NewProvider(ctx, "explicit", nil)
		if err != nil {
			return err
		}
		withProviders, err := NewLocal(ctx, "withProviders", nil, pulumi.ProviderMap(map[string]pulumi.ProviderResource{
			"simple": explicit,
		}))
		if err != nil {
			return err
		}
		ctx.Export("result", withProviders.Result)
		return nil
	})
}
