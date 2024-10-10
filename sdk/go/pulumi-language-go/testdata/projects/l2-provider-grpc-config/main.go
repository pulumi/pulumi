package main

import (
	"example.com/pulumi-testconfigprovider/sdk/go/testconfigprovider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The schema provider covers interesting schema shapes.
		schemaprov, err := testconfigprovider.NewProvider(ctx, "schemaprov", &testconfigprovider.ProviderArgs{
			S1:  pulumi.String(""),
			S2:  pulumi.String("x"),
			S3:  pulumi.String("{}"),
			I1:  pulumi.Int(0),
			I2:  pulumi.Int(42),
			N1:  pulumi.Float64(0),
			N2:  pulumi.Float64(42.42),
			B1:  pulumi.Bool(true),
			B2:  pulumi.Bool(false),
			Ls1: pulumi.StringArray{},
			Ls2: pulumi.StringArray{
				pulumi.String(""),
				pulumi.String("foo"),
			},
			Li1: pulumi.IntArray{
				pulumi.Int(1),
				pulumi.Int(2),
			},
			Ms1: pulumi.StringMap{},
			Ms2: pulumi.StringMap{
				"key1": pulumi.String("value1"),
				"key2": pulumi.String("value2"),
			},
			Mi1: pulumi.IntMap{
				"key1": pulumi.Int(0),
				"key2": pulumi.Int(42),
			},
			Os1: &testconfigprovider.Ts1Args{},
			Os2: &testconfigprovider.Ts2Args{
				X: pulumi.String("x-value"),
			},
			Oi1: &testconfigprovider.Ti1Args{
				X: pulumi.Int(42),
			},
		})
		if err != nil {
			return err
		}
		_, err = testconfigprovider.NewConfigGetter(ctx, "schemaconf", nil, pulumi.Provider(schemaprov))
		if err != nil {
			return err
		}
		return nil
	})
}
