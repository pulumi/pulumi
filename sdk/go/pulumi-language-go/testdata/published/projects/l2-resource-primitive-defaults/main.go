package main

import (
	"example.com/pulumi-primitive-defaults/sdk/go/v8/primitivedefaults"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := primitivedefaults.NewResource(ctx, "resExplicit", &primitivedefaults.ResourceArgs{
			Boolean: pulumi.Bool(true),
			Float:   pulumi.Float64(3.14),
			Integer: pulumi.Int(42),
			String:  pulumi.String("hello"),
		})
		if err != nil {
			return err
		}
		_, err = primitivedefaults.NewResource(ctx, "resDefaulted", nil)
		if err != nil {
			return err
		}
		return nil
	})
}
