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
