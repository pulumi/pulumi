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
		stackRef, err := pulumi.NewStackReference(ctx, slug, nil)

		if err != nil {
			return errors.Wrap(err, "Error reading stack reference.")
		}

		val := stackRef.GetOutput(pulumi.String("val"))
		if len(val) != 2 || val[0] != pulumi.String("a") || val[1] != pulumi.String("b") {
			return errors.Errorf("Invalid result")
		}
		ctx.Export("val",
			pulumi.StringArray([]pulumi.StringInput{pulumi.String("a"), pulumi.String("b")}))

		return nil
	})
}
