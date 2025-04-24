package main

import (
	"example.com/pulumi-goodbye/sdk/go/v2/goodbye"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := goodbye.NewProvider(ctx, "prov", &goodbye.ProviderArgs{
			Text: pulumi.String("World"),
		})
		if err != nil {
			return err
		}
		// The resource name is based on the parameter value
		res, err := goodbye.NewGoodbye(ctx, "res", nil, pulumi.Provider(prov))
		if err != nil {
			return err
		}
		ctx.Export("parameterValue", res.ParameterValue)
		return nil
	})
}
