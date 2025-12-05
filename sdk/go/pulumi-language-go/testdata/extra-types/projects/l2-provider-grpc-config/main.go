package main

import (
	"example.com/pulumi-config-grpc/sdk/go/configgrpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Cover interesting schema shapes.
		config_grpc_provider, err := configgrpc.NewProvider(ctx, "config_grpc_provider", &configgrpc.ProviderArgs{
			String1:     pulumi.String(""),
			String2:     pulumi.String("x"),
			String3:     pulumi.String("{}"),
			Int1:        pulumi.Int(0),
			Int2:        pulumi.Int(42),
			Num1:        pulumi.Float64(0),
			Num2:        pulumi.Float64(42.42),
			Bool1:       pulumi.Bool(true),
			Bool2:       pulumi.Bool(false),
			ListString1: pulumi.StringArray{},
			ListString2: pulumi.StringArray{
				pulumi.String(""),
				pulumi.String("foo"),
			},
			ListInt1: pulumi.IntArray{
				pulumi.Int(1),
				pulumi.Int(2),
			},
			MapString1: pulumi.StringMap{},
			MapString2: pulumi.StringMap{
				"key1": pulumi.String("value1"),
				"key2": pulumi.String("value2"),
			},
			MapInt1: pulumi.IntMap{
				"key1": pulumi.Int(0),
				"key2": pulumi.Int(42),
			},
			ObjString1: &configgrpc.Tstring1Args{},
			ObjString2: &configgrpc.Tstring2Args{
				X: pulumi.String("x-value"),
			},
			ObjInt1: &configgrpc.Tint1Args{
				X: pulumi.Int(42),
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
