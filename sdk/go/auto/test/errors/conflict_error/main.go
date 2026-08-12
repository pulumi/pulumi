//go:build !all
// +build !all

package main

import (
	"os"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Block until the test removes the file named by PULUMI_TEST_BLOCK_FILE. This lets the test
		// deterministically hold one update inside the program while a second update races to produce a
		// concurrent update error, instead of relying on a timing-sensitive sleep.
		if block := os.Getenv("PULUMI_TEST_BLOCK_FILE"); block != "" {
			for {
				if _, err := os.Stat(block); os.IsNotExist(err) {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
}
