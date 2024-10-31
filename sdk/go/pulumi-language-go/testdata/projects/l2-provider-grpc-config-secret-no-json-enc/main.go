package main

import (
	"example.com/pulumi-config-grpc-no-jsonenc/sdk/go/configgrpcnojsonenc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// This provider covers scenarios where user passes secret values to the provider.
		config_grpc_provider, err := configgrpcnojsonenc.NewProvider(ctx, "config_grpc_provider", &configgrpcnojsonenc.ProviderArgs{
			String1: configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
				String1: pulumi.String("SECRET"),
			}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (string, error) {
				return invoke.String1, nil
			}).(pulumi.StringOutput),
			Int1: configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
				Int1: pulumi.Int(1234567890),
			}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (int, error) {
				return invoke.Int1, nil
			}).(pulumi.IntOutput),
			Num1: configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
				Num1: pulumi.Float64(123456.789),
			}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (float64, error) {
				return invoke.Num1, nil
			}).(pulumi.Float64Output),
			Bool1: configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
				Bool1: pulumi.Bool(true),
			}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (bool, error) {
				return invoke.Bool1, nil
			}).(pulumi.BoolOutput),
			ListString1: configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
				ListString1: pulumi.StringArray{
					pulumi.String("SECRET"),
					pulumi.String("SECRET2"),
				},
			}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) ([]string, error) {
				return invoke.ListString1, nil
			}).(pulumi.StringArrayOutput),
			ListString2: pulumi.StringArray{
				pulumi.String("VALUE"),
				configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
					String1: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (string, error) {
					return invoke.String1, nil
				}).(pulumi.StringOutput),
			},
			MapString2: pulumi.StringMap{
				"key1": pulumi.String("value1"),
				"key2": configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
					String1: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (string, error) {
					return invoke.String1, nil
				}).(pulumi.StringOutput),
			},
			ObjString2: &configgrpcnojsonenc.Tstring2Args{
				X: configgrpcnojsonenc.ToSecretOutput(ctx, configgrpcnojsonenc.ToSecretOutputArgs{
					String1: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke configgrpcnojsonenc.ToSecretResult) (string, error) {
					return invoke.String1, nil
				}).(pulumi.StringOutput),
			},
		})
		if err != nil {
			return err
		}
		_, err = configgrpcnojsonenc.NewConfigFetcher(ctx, "config", nil, pulumi.Provider(config_grpc_provider))
		if err != nil {
			return err
		}
		return nil
	})
}
