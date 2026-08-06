package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func notImplemented(message string) any {
	panic(message)
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("result", notImplemented("expression here is not implemented yet").(pulumi.Any))
		return nil
	})
}
