package main

import (
	"example.com/pulumi-output/sdk/go/v23/output"
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simple.NewResource(ctx, "replacementTrigger", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplacementTrigger(pulumi.Any("test2")))
		if err != nil {
			return err
		}
		unknown, err := output.NewResource(ctx, "unknown", &output.ResourceArgs{
			Value: pulumi.Float64(2),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "unknownReplacementTrigger", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplacementTrigger(pulumi.Any(unknown.Output)))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "notReplacementTrigger", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "secretReplacementTrigger", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.ReplacementTrigger(pulumi.Any(interface{}(pulumi.ToSecret([]float64{
			3,
			2,
			1,
		}).(pulumi.Float64ArrayOutput)))))
		if err != nil {
			return err
		}
		return nil
	})
}
