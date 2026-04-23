package main

import (
	"example.com/pulumi-configurer/sdk/go/v38/configurer"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		configurer2, err := configurer.NewConfigurer(ctx, "configurer", &configurer.ConfigurerArgs{
			ProviderConfig: pulumi.String("propagated"),
		})
		if err != nil {
			return err
		}
		callPlainProvider, err := configurer2.PlainProvider(ctx)
		if err != nil {
			return err
		}
		_, err = configurer.NewCustom(ctx, "customFromPlainProvider", &configurer.CustomArgs{
			Value: pulumi.String("from-plain-provider"),
		}, pulumi.Provider(callPlainProvider))
		if err != nil {
			return err
		}
		callNestedPlainProvider1, err := configurer2.NestedPlainProvider(ctx)
		if err != nil {
			return err
		}
		_, err = configurer.NewCustom(ctx, "customFromNestedPlainProvider", &configurer.CustomArgs{
			Value: pulumi.String("from-nested-plain-provider"),
		}, pulumi.Provider(callNestedPlainProvider1.Provider))
		if err != nil {
			return err
		}
		callPlainValue2, err := configurer2.PlainValue(ctx)
		if err != nil {
			return err
		}
		ctx.Export("plainValue", pulumi.Int(callPlainValue2))
		callNestedPlainProvider3, err := configurer2.NestedPlainProvider(ctx)
		if err != nil {
			return err
		}
		ctx.Export("nestedPlainValue", pulumi.Int(callNestedPlainProvider3.Value))
		return nil
	})
}
