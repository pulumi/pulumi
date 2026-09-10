package main

import (
	"example.com/pulumi-constant/sdk/go/v43/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		first, err := constant.NewResource(ctx, "first", &constant.ResourceArgs{
			Kind:  pulumi.String("Constant"),
			Flag:  pulumi.Bool(true),
			Count: pulumi.Int(3),
			Ratio: pulumi.Float64(1.5),
		})
		if err != nil {
			return err
		}
		ctx.Export("kind", first.Kind)
		ctx.Export("flag", first.Flag)
		ctx.Export("count", first.Count)
		ctx.Export("ratio", first.Ratio)
		return nil
	})
}
