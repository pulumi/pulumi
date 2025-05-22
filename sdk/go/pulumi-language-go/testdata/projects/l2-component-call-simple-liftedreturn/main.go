package main

import (
	"example.com/pulumi-componentreturnscalar/sdk/go/v18/componentreturnscalar"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		component1, err := componentreturnscalar.NewComponentCallable(ctx, "component1", &componentreturnscalar.ComponentCallableArgs{
			Value: pulumi.String("bar"),
		})
		if err != nil {
			return err
		}
		callIdentity, err := "TODO: call call"
		if err != nil {
			return err
		}
		ctx.Export("from_identity", callIdentity)
		callPrefixed1, err := "TODO: call call"
		if err != nil {
			return err
		}
		ctx.Export("from_prefixed", callPrefixed1)
		return nil
	})
}
