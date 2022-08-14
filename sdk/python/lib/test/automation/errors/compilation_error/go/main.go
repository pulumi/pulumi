package main

import (
	"github.com/mariospas/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		will not compile
		return nil
	})
}
