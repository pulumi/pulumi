package main

import (
	"fmt"

	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		extraCount := cfg.RequireInt("extraCount")
		_, err := simple.NewResource(ctx, "res1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		var res2 []*simple.Resource
		for index := 0; index < extraCount; index++ {
			key0 := index
			__res, err := simple.NewResource(ctx, fmt.Sprintf("res2-%v", key0), &simple.ResourceArgs{
				Value: pulumi.Bool(false),
			})
			if err != nil {
				return err
			}
			res2 = append(res2, __res)
		}
		return nil
	})
}
