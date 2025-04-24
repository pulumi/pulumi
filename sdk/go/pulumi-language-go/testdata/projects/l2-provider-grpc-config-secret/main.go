package main

import (
	"example.com/pulumi-config-grpc/sdk/go/configgrpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// This provider covers scenarios where user passes secret values to the provider.
		config_grpc_provider, err := configgrpc.NewProvider(ctx, "config_grpc_provider", &configgrpc.ProviderArgs{
			String1: configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
				String1: pulumi.String("SECRET"),
			}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (string, error) {
				return invoke.String1, nil
			}).(pulumi.StringOutput),
			Int1: configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
				Int1: pulumi.Int(1234567890),
			}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (int, error) {
				return invoke.Int1, nil
			}).(pulumi.IntOutput),
			Num1: configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
				Num1: pulumi.Float64(123456.789),
			}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (float64, error) {
				return invoke.Num1, nil
			}).(pulumi.Float64Output),
			Bool1: configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
				Bool1: pulumi.Bool(true),
			}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (bool, error) {
				return invoke.Bool1, nil
			}).(pulumi.BoolOutput),
			ListString1: configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
				ListString1: pulumi.StringArray{
					pulumi.String("SECRET"),
					pulumi.String("SECRET2"),
				},
			}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) ([]string, error) {
				return invoke.ListString1, nil
			}).(pulumi.StringArrayOutput),
			ListString2: pulumi.StringArray{
				pulumi.String("VALUE"),
				configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
					String1: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (string, error) {
					return invoke.String1, nil
				}).(pulumi.StringOutput),
			},
			MapString2: pulumi.StringMap{
				"key1": pulumi.String("value1"),
				"key2": configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
					String1: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (string, error) {
					return invoke.String1, nil
				}).(pulumi.StringOutput),
			},
			ObjString2: &configgrpc.Tstring2Args{
				X: configgrpc.ToSecretOutput(ctx, configgrpc.ToSecretOutputArgs{
					String1: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke configgrpc.ToSecretResult) (string, error) {
					return invoke.String1, nil
				}).(pulumi.StringOutput),
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
