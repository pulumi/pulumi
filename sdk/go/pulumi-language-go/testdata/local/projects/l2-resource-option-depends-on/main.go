package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		noDependsOn, err := simple.NewResource(ctx, "noDependsOn", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withDependsOn", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		}, pulumi.DependsOn([]pulumi.Resource{
			noDependsOn,
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
