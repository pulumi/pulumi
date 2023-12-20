// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		slug := fmt.Sprintf("%v/%v/%v", ctx.Organization(), ctx.Project(), ctx.Stack())
		_, err := pulumi.NewStackReference(ctx, slug, nil)
		if err != nil {
			return fmt.Errorf("error reading stack reference: %w", err)
		}
		ctx.Export("val",
			pulumi.StringArray([]pulumi.StringInput{pulumi.String("a"), pulumi.String("b")}))

		return nil
	})
}
