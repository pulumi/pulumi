package main

import (
	"example.com/pulumi-component/sdk/go/v13/component"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		explicit, err := component.NewProvider(ctx, "explicit", nil)
		if err != nil {
			return err
		}
		_, err = component.NewComponentCallable(ctx, "list", &component.ComponentCallableArgs{
			Value: pulumi.String("value"),
		}, pulumi.Provider(explicit))
		if err != nil {
			return err
		}
		_, err = component.NewComponentCallable(ctx, "map", &component.ComponentCallableArgs{
			Value: pulumi.String("value"),
		}, pulumi.ProviderMap(map[string]pulumi.ProviderResource{
			"component": explicit,
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
