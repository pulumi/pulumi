package main

import (
	"example.com/pulumi-call/sdk/go/v15/call"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		defaultRes, err := call.NewCustom(ctx, "defaultRes", &call.CustomArgs{
			Value: pulumi.String("defaultValue"),
		})
		if err != nil {
			return err
		}

		defaultProviderValue, err := defaultRes.ProviderValue(ctx)
		if err != nil {
			return err
		}

		ctx.Export("defaultProviderValue", defaultProviderValue.Result())
		return nil
	})
}
