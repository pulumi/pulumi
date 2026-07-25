package main

import (
	"example.com/pulumi-inheritderived/sdk/go/inheritderived"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		derived, err := inheritderived.NewDerivedComponent(ctx, "derived", &inheritderived.DerivedComponentArgs{
			Message: pulumi.String("hello"),
			Scale:   pulumi.Int(7),
		})
		if err != nil {
			return err
		}
		ctx.Export("baseOutput", derived.BaseOutput)
		ctx.Export("derivedOutput", derived.DerivedOutput)
		return nil
	})
}
