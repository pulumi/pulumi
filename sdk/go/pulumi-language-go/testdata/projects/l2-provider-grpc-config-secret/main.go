package main

import (
	"example.com/pulumi-testconfigprovider/sdk/go/testconfigprovider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The program_secret_provider covers scenarios where user passes secret values to the provider.
		programsecretprov, err := testconfigprovider.NewProvider(ctx, "programsecretprov", &testconfigprovider.ProviderArgs{
			S1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
				S: pulumi.String("SECRET"),
			}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
				return invoke.S, nil
			}).(pulumi.StringOutput),
			I1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
				I: pulumi.Int(1234567890),
			}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (int, error) {
				return invoke.I, nil
			}).(pulumi.IntOutput),
			N1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
				N: pulumi.Float64(123456.789),
			}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (float64, error) {
				return invoke.N, nil
			}).(pulumi.Float64Output),
			B1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
				B: pulumi.Bool(true),
			}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (bool, error) {
				return invoke.B, nil
			}).(pulumi.BoolOutput),
			Ls1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
				Ls: pulumi.StringArray{
					pulumi.String("SECRET"),
					pulumi.String("SECRET2"),
				},
			}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) ([]string, error) {
				return invoke.Ls, nil
			}).(pulumi.StringArrayOutput),
			Ls2: pulumi.StringArray{
				pulumi.String("VALUE"),
				testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
					S: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
					return invoke.S, nil
				}).(pulumi.StringOutput),
			},
			Ms2: pulumi.StringMap{
				"key1": pulumi.String("value1"),
				"key2": testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
					S: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
					return invoke.S, nil
				}).(pulumi.StringOutput),
			},
			Os2: &testconfigprovider.Ts2Args{
				X: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
					S: pulumi.String("SECRET"),
				}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
					return invoke.S, nil
				}).(pulumi.StringOutput),
			},
		})
		if err != nil {
			return err
		}
		_, err = testconfigprovider.NewConfigGetter(ctx, "programsecretconf", nil, pulumi.Provider(programsecretprov))
		if err != nil {
			return err
		}
		return nil
	})
}
