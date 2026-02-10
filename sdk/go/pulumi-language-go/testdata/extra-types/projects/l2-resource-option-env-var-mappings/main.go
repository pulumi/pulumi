package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simple.NewProvider(ctx, "prov", nil, pulumi.EnvVarMappings(map[string]string{"MY_VAR": "PROVIDER_VAR", "OTHER_VAR": "TARGET_VAR"}))
		if err != nil {
			return err
		}
		return nil
	})
}
