package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var numbers []*random.RandomInteger
		for index := 0; index < 2; index++ {
			key0 := index
			val0 := index
			__res, err := random.NewRandomInteger(ctx, fmt.Sprintf("numbers-%v", key0), &random.RandomIntegerArgs{
				Min:  pulumi.Int(1),
				Max:  pulumi.Int(val0),
				Seed: fmt.Sprintf("seed%v", val0),
			})
			if err != nil {
				return err
			}
			numbers = append(numbers, __res)
		}
		ctx.Export("first", numbers[0].ID())
		ctx.Export("second", numbers[1].ID())
		return nil
	})
}
