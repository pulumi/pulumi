package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple/overlapping_pkg"
	simpleoverlapoverlapping_pkg "example.com/pulumi-simpleoverlap/sdk/go/v2/simpleoverlap/overlapping_pkg"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		one, err := overlapping_pkg.NewResource(ctx, "one", &overlapping_pkg.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		two, err := overlapping_pkg.NewOverlapResource(ctx, "two", &overlapping_pkg.OverlapResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("outOne", one)
		ctx.Export("outTwo", two.Value)
		return nil
	})
}
