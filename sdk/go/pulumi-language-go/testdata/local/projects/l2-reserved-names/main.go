package main

import (
	"example.com/pulumi-reservednames/sdk/go/v51/reservednames"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// A resource whose `elementType` property collides with the `ElementType()` method that
		// generated Go SDK types must implement.
		elem, err := reservednames.NewElementType(ctx, "elem", &reservednames.ElementTypeArgs{
			ElementType_: &reservednames.ElementTypeTypeArgs{
				ElementType_: pulumi.String("nested"),
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("elementType", elem.ElementType_)
		ctx.Export("nested", elem.ElementType_.GetElementType_())
		return nil
	})
}
