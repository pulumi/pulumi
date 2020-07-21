package main

import (
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
}
