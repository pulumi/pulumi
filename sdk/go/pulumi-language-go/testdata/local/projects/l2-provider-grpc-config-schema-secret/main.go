package main

import (
	"example.com/pulumi-config-grpc/sdk/go/configgrpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// This provider covers scenarios where configuration properties are marked as secret in the schema.
		config_grpc_provider, err := configgrpc.NewProvider(ctx, "config_grpc_provider", &configgrpc.ProviderArgs{
			SecretString1: pulumi.String("SECRET"),
			SecretInt1:    pulumi.Int(16),
			SecretNum1:    pulumi.Float64(123456.789),
			SecretBool1:   pulumi.Bool(true),
			ListSecretString1: pulumi.StringArray{
				pulumi.String("SECRET"),
				pulumi.String("SECRET2"),
			},
			MapSecretString1: pulumi.StringMap{
				"key1": pulumi.String("SECRET"),
				"key2": pulumi.String("SECRET2"),
			},
			ObjSecretString1: &configgrpc.TsecretString1Args{
				SecretX: pulumi.String("SECRET"),
			},
		})
		if err != nil {
			return err
		}
		_, err = configgrpc.NewConfigFetcher(ctx, "config", nil, pulumi.Provider(config_grpc_provider))
		if err != nil {
			return err
		}
		return nil
	})
}
