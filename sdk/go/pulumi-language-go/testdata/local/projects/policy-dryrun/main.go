package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res1, err := simple.NewResource(ctx, "res1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "res2", &simple.ResourceArgs{
			Value: res1.Value.ApplyT(func(value bool) (bool, error) {
				return !value, nil
			}).(pulumi.BoolOutput),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
