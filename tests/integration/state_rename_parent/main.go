package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		parent, err := NewRandom(ctx, "parent", &RandomArgs{
			Length: pulumi.Int(8),
		})
		if err != nil {
			return err
		}

		child, err := NewRandom(ctx, "child", &RandomArgs{
			Length: pulumi.Int(4),
		}, pulumi.Parent(parent))
		if err != nil {
			return err
		}

		ctx.Export("parent", parent.Result)
		ctx.Export("child", child.Result)
		return nil
	})
}
