package main

import (
	"example.com/pulumi-constant/sdk/go/v43/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		first, err := constant.NewResource(ctx, "first", &constant.ResourceArgs{
			Kind: pulumi.String("Constant"),
		})
		if err != nil {
			return err
		}
		ctx.Export("kind", first.Kind)
		return nil
	})
}
