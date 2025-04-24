package main

import (
	"example.com/pulumi-call/sdk/go/v15/call"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		explicitProv, err := call.NewProvider(ctx, "explicitProv", &call.ProviderArgs{
			Value: pulumi.String("explicitProvValue"),
		})
		if err != nil {
			return err
		}

		explicitRes, err := call.NewCustom(ctx, "explicitRes", &call.CustomArgs{
			Value: pulumi.String("explicitValue"),
		}, pulumi.Provider(explicitProv))
		if err != nil {
			return err
		}

		explicitProviderValue, err := explicitRes.ProviderValue(ctx)
		if err != nil {
			return err
		}

		explicitProvFromIdentity, err := explicitProv.Identity(ctx)
		if err != nil {
			return err
		}

		explicitProvFromPrefixed, err := explicitProv.Prefixed(ctx, &call.ProviderPrefixedArgs{
			Prefix: pulumi.String("call-prefix-"),
		})
		if err != nil {
			return err
		}

		ctx.Export("explicitProviderValue", explicitProviderValue.Result())
		ctx.Export("explicitProvFromIdentity", explicitProvFromIdentity.Result())
		ctx.Export("explicitProvFromPrefixed", explicitProvFromPrefixed.Result())
		return nil
	})
}
