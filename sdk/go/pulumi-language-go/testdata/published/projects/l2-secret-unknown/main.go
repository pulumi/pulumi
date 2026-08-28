package main

import (
	"example.com/pulumi-output/sdk/go/v23/output"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		r, err := output.NewResource(ctx, "r", &output.ResourceArgs{
			Value: pulumi.Float64(1),
		})
		if err != nil {
			return err
		}
		ctx.Export("wrapped", pulumi.ToSecret(r.Output).(pulumi.StringOutput))
		return nil
	})
}
