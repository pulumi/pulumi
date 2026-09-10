// Copyright 2026, Pulumi Corporation.  All rights reserved.

package main

import (
	"example.com/pulumi-pkg/sdk/go/pkg"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// TestSourcePositionGo asserts the line numbers of the two calls below. Update the expected lines
// in that test if you move them.
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := pkg.NewRandom(ctx, "reg", &pkg.RandomArgs{Length: pulumi.Int(8)})
		if err != nil {
			return err
		}

		_, err = pkg.GetRandom(ctx, "read", pulumi.ID("abcdefgh"), nil)
		return err
	})
}
