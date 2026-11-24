package main

import (
	"example.com/pulumi-unionref/sdk/go/v63/unionref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		x, err := unionref.NewXType(ctx, "x", nil)
		if err != nil {
			return err
		}
		y, err := unionref.NewYType(ctx, "y", nil)
		if err != nil {
			return err
		}
		_, err = unionref.NewZType(ctx, "z", &unionref.ZTypeArgs{
			Value: x,
			Wrapper: &unionref.WrapperArgs{
				Value: y,
			},
			Values: pulumi.Array{
				x,
				y,
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
