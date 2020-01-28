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

		val := pulumi.StringArrayOutput(stackRef.GetOutput(pulumi.String("val")))

		errChan := make(chan error, 0)
		results := make(chan []string, 0)

		_ = val.ApplyStringArray(func(v []string) ([]string, error) {
			if len(v) != 2 || v[0] != "a" || v[1] != "b" {
				errChan <- errors.Errorf("Invalid result")
				return nil, errors.Errorf("Invalid result")
			}
			results <- v
			return v, nil
		})

		select {
		case err = <-errChan:
			return err
		case _ = <-results:
			return nil
		}
	})
}
