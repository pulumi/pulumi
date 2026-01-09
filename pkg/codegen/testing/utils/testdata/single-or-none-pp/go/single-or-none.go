package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func singleOrNone[T any](elements []T) T {
	if len(elements) != 1 {
		panic(fmt.Errorf("singleOrNone expected input slice to have a single element"))
	}
	return elements[0]
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("result", pulumi.Float64(singleOrNone([]float64{
			1,
		})))
		return nil
	})
}
