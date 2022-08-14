package main

import (
	"time"

	"github.com/mariospas/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		time.Sleep(1 * time.Second)
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
}
