package main

import (
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		time.Sleep(10 * time.Second)
		return nil
	})
}
