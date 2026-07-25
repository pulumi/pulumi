package main

import (
	"example.com/pulumi-inherit/sdk/go/inherit"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		derived, err := inherit.NewDerived(ctx, "derived", &inherit.DerivedArgs{
			Message: pulumi.String("hello"),
			Scale:   pulumi.Int(3),
		})
		if err != nil {
			return err
		}
		ctx.Export("baseOutput", derived.BaseOutput)
		ctx.Export("derivedOutput", derived.DerivedOutput)
		return nil
	})
}
