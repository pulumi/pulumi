// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, ctx.Project())

		org := cfg.Require("org")
		slug := fmt.Sprintf("%v/%v/%v", org, ctx.Project(), ctx.Stack())
		_, err := pulumi.NewStackReference(ctx, slug, nil)

		if err != nil {
			return errors.Wrap(err, "Error reading stack reference.")
		}
		ctx.Export("val",
			pulumi.StringArray([]pulumi.StringInput{pulumi.String("a"), pulumi.String("b")}))

		return nil
	})
}
