// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Motivating case: an AND reducer where true sticks to false. Once the reduced output
// becomes false, it cannot flip back to true even if the program's input is true again.
// On create oldOutput is nil; treat that as the identity for &&.
func reduce(_oldInput, oldOutput, newInput any) (any, error) {
	if oldOutput == nil {
		return newInput, nil
	}
	o, _ := oldOutput.(bool)
	n, _ := newInput.(bool)
	return o && n, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		stash, err := pulumi.NewStash(ctx, "bucket", &pulumi.StashArgs{
			Input:  pulumi.Any(true),
			Reduce: reduce,
		})
		if err != nil {
			return err
		}
		ctx.Export("input", stash.Input)
		ctx.Export("output", stash.Output)
		return nil
	})
}
