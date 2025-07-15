package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res1, err := NewRandom(ctx, "res1", &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}

		res2, err := NewRandom(ctx, "res2", &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}

		ctx.Export("res1", res1.URN())
		ctx.Export("res2", res2.URN())
		return nil
	})
}
