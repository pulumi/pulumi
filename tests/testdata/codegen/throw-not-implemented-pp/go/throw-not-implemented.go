package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func notImplemented(message string) pulumi.AnyOutput {
	panic(message)
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("result", pulumi.Any(notImplemented("expression here is not implemented yet")))
		return nil
	})
}
