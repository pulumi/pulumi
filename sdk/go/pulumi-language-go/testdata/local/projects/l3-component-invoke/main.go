package main

import (
	"example.com/pulumi-config/sdk/go/v9/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := config.NewProvider(ctx, "prov", &config.ProviderArgs{
			Name: pulumi.String("my config"),
		})
		if err != nil {
			return err
		}
		_, err = NewInvokeComponent(ctx, "myComponent", nil, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		return nil
	})
}
