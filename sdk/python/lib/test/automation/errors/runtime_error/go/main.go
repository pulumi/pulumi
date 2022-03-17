package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var x []string
		ctx.Export("a", pulumi.String(x[0]))
		return nil
	})
}
