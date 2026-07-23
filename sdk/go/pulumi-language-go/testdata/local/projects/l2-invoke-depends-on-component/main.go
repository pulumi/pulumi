package main

import (
	"example.com/pulumi-component/sdk/go/v13/component"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		target, err := component.NewComponentCustomRefOutput(ctx, "target", &component.ComponentCustomRefOutputArgs{
			Value: pulumi.String("checked"),
		})
		if err != nil {
			return err
		}
		data := component.IdentityOutput(ctx, component.IdentityOutputArgs{
			Input: pulumi.String("reachable"),
		}, pulumi.DependsOn([]pulumi.Resource{
			pulumi.Resource(target),
		}))
		ctx.Export("echoed", data.ApplyT(func(data component.IdentityResult) (string, error) {
			return data.Result, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
