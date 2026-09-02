// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Once the reduced output is false, flipping input back to true must not resurrect it:
// reducer(false, false, true) == false.
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		stash, err := pulumi.NewStash(ctx, "bucket", &pulumi.StashArgs{
			Input: pulumi.Any(true),
			Reduce: func(_oldInput, oldOutput, newInput any) (any, error) {
				o, _ := oldOutput.(bool)
				n, _ := newInput.(bool)
				return o && n, nil
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("input", stash.Input)
		ctx.Export("output", stash.Output)
		return nil
	})
}
