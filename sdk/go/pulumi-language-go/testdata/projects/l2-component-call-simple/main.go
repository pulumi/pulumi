package main

import (
	"example.com/pulumi-component/sdk/go/v13/component"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		component1, err := component.NewComponentCallable(ctx, "component1", &component.ComponentCallableArgs{
			Value: pulumi.String("bar"),
		})
		if err != nil {
			return err
		}
		callIdentity, err := component1.Identity(ctx)
		if err != nil {
			return err
		}
		ctx.Export("from_identity", pulumi.String(callIdentity.Result))
		callPrefixed1, err := component1.Prefixed(ctx, &component.ComponentCallablePrefixedArgs{
			Prefix: "foo-",
		})
		if err != nil {
			return err
		}
		ctx.Export("from_prefixed", pulumi.String(callPrefixed1.Result))
		return nil
	})
}
