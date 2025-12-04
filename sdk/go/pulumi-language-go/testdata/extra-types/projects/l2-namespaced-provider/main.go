package main

import (
	"example.com/pulumi-component/sdk/go/v13/component"
	"github.com/a-namespace/pulumi-namespaced/sdk/go/v16/namespaced"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		componentRes, err := component.NewComponentCustomRefOutput(ctx, "componentRes", &component.ComponentCustomRefOutputArgs{
			Value: pulumi.String("foo-bar-baz"),
		})
		if err != nil {
			return err
		}
		_, err = namespaced.NewResource(ctx, "res", &namespaced.ResourceArgs{
			Value:       pulumi.Bool(true),
			ResourceRef: componentRes.Ref,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
