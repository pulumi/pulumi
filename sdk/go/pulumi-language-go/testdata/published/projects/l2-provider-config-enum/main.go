package main

import (
	"example.com/pulumi-config-enum/sdk/go/v40/configenum"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := configenum.NewProvider(ctx, "prov", &configenum.ProviderArgs{
			AString: pulumi.String("hello"),
			AEnum:   configenum.MyEnumTwo,
		})
		if err != nil {
			return err
		}
		// Reference the provider's outputs - including the enum - from another resource.
		_, err = configenum.NewResource(ctx, "res", &configenum.ResourceArgs{
			TheString: prov.AString,
			TheEnum:   prov.AEnum,
		}, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		return nil
	})
}
