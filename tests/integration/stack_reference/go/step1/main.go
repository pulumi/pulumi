// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		cfg := config.New(ctx, ctx.Project())

		org := cfg.Require("org")
		slug := fmt.Sprintf("%v/%v/%v", org, ctx.Project(), ctx.Stack())
		stackRef, err := pulumi.NewStackReference(ctx, slug, nil)

		if err != nil {
			return fmt.Errorf("error reading stack reference: %v", err)
		}

		val := pulumi.StringArrayOutput(stackRef.GetOutput(pulumi.String("val")))

		errChan := make(chan error)
		results := make(chan []string)

		_ = val.ApplyT(func(v []string) ([]string, error) {
			if len(v) != 2 || v[0] != "a" || v[1] != "b" {
				errChan <- fmt.Errorf("invalid result")
				return nil, fmt.Errorf("invalid result")
			}
			results <- v
			return v, nil
		})
		ctx.Export("val2", pulumi.ToSecret(val))

		select {
		case err = <-errChan:
			return err
		case <-results:
			return nil
		}
	})
}
