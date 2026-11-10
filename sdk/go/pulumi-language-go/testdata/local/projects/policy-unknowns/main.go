package main

import (
	"example.com/pulumi-output/sdk/go/v23/output"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Produces outputs that are unknown during preview via the output provider.
		src, err := output.NewResource(ctx, "src", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		})
		if err != nil {
			return err
		}
		// value is an unknown bool during preview, known-true on update. The policy
		// pack should "skip" (report not-applicable) for this resource during preview.
		_, err = simple.NewResource(ctx, "unknownRes", &simple.ResourceArgs{
			Value: src.Output.ApplyT(func(output string) (bool, error) {
				return output == "hello", nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		// value is a secret-wrapped unknown bool during preview, secret-wrapped true on
		// update. The policy pack should also "skip" during preview, and must be able
		// to unwrap the secret and inspect it on update.
		_, err = simple.NewResource(ctx, "secretUnknownRes", &simple.ResourceArgs{
			Value: src.SecretOutput.ApplyT(func(secretOutput string) (bool, error) {
				return secretOutput == "hello", nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		// value is a known plain bool. The policy pack inspects it directly.
		_, err = simple.NewResource(ctx, "knownRes", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		// value is a known secret-wrapped bool. The policy pack must unwrap the secret
		// to inspect the underlying value.
		_, err = simple.NewResource(ctx, "secretKnownRes", &simple.ResourceArgs{
			Value: pulumi.ToSecret(false).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
