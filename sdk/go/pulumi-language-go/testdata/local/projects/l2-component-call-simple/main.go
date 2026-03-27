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
		ctx.Export("from_identity", callIdentity.ApplyT(func(call component.ComponentCallableIdentityResult) (string, error) {
			return call.Result, nil
		}).(pulumi.StringOutput))
		callPrefixed1, err := component1.Prefixed(ctx, &component.ComponentCallablePrefixedArgs{
			Prefix: pulumi.String("foo-"),
		})
		if err != nil {
			return err
		}
		ctx.Export("from_prefixed", callPrefixed1.ApplyT(func(call component.ComponentCallablePrefixedResult) (string, error) {
			return call.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
