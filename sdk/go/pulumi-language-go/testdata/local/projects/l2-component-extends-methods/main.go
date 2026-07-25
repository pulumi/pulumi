package main

import (
	"example.com/pulumi-inherit/sdk/go/inherit"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		derived, err := inherit.NewDerived(ctx, "derived", &inherit.DerivedArgs{
			Message: pulumi.String("hi"),
			Scale:   pulumi.Int(1),
		})
		if err != nil {
			return err
		}
		callGetStatus, err := derived.GetStatus(ctx)
		if err != nil {
			return err
		}
		ctx.Export("status", callGetStatus.ApplyT(func(call inherit.BaseGetStatusResult) (string, error) {
			return call.Status, nil
		}).(pulumi.StringOutput))
		return nil
	})
}
