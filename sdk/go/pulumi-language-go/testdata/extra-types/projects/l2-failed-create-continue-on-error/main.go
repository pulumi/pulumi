package main

import (
	"example.com/pulumi-fail_on_create/sdk/go/v4/fail_on_create"
	"example.com/pulumi-simple/sdk/go/v2/simple"
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
		_, err = simple.NewResource(ctx, "dependent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.DependsOn([]pulumi.Resource{
			failing,
		}))
		if err != nil {
			return err
		}
		dependent_on_output, err := simple.NewResource(ctx, "dependent_on_output", &simple.ResourceArgs{
			Value: failing.Value,
		})
		if err != nil {
			return err
		}
		independent, err := simple.NewResource(ctx, "independent", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "double_dependency", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.DependsOn([]pulumi.Resource{
			independent,
			dependent_on_output,
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
