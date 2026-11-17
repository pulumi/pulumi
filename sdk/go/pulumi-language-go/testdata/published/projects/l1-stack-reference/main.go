package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ref, err := pulumi.NewStackReference(ctx, "ref", &pulumi.StackReferenceArgs{
			Name: pulumi.String("organization/other/dev"),
		})
		if err != nil {
			return err
		}
		ctx.Export("plain", ref.GetOutput(pulumi.String("plain")))
		ctx.Export("secret", ref.GetOutput(pulumi.String("secret")))
		ctx.Export("secret_unsecret", pulumi.Unsecret(ref.GetOutput(pulumi.String("secret"))).(pulumi.AnyOutput))
		return nil
	})
}
