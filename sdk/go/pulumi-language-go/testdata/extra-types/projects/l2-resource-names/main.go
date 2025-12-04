package main

import (
	"example.com/pulumi-names/sdk/go/v6/names"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := names.NewResMap(ctx, "res1", &names.ResMapArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = names.NewResArray(ctx, "res2", &names.ResArrayArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = names.NewResList(ctx, "res3", &names.ResListArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = names.NewResResource(ctx, "res4", &names.ResResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
