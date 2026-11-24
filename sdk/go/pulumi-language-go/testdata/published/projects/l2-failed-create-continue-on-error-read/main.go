package main

import (
	"fmt"

	"example.com/pulumi-fail_on_create/sdk/go/v4/fail_on_create"
	"example.com/pulumi-read/sdk/go/v39/read"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		failing, err := fail_on_create.NewResource(ctx, "failing", &fail_on_create.ResourceArgs{
			Value: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		res, err := read.GetResource(ctx, "res", pulumi.ID("existing-id"), &read.ResourceState{
			Lookup: pulumi.String("existing-key"),
		}, pulumi.DependsOn([]pulumi.Resource{
			failing,
		}))
		if err != nil {
			return err
		}
		res_prop_dep, err := read.GetResource(ctx, "res_prop_dep", pulumi.ID("existing-id"), &read.ResourceState{
			Lookup: failing.Value.ApplyT(func(value bool) (string, error) {
				return fmt.Sprintf("existing-key-%v", value), nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}
		ctx.Export("readValue", res.Value)
		ctx.Export("readPropDepValue", res_prop_dep.Value)
		return nil
	})
}
