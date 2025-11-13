package main

import (
	"example.com/pulumi-large/sdk/go/v4/large"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := large.NewString(ctx, "res", &large.StringArgs{
			Value: pulumi.String("hello world"),
		})
		if err != nil {
			return err
		}
		ctx.Export("output", res.Value)
		return nil
	})
}
