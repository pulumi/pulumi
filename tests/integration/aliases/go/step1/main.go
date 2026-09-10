// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		for _, scenario := range []func(*pulumi.Context) error{
			scenarioRename,
			scenarioAdoptIntoComponent,
			scenarioRenameComponentAndChild,
			scenarioRetypeComponent,
			scenarioRenameComponent,
			scenarioRetypeParents,
			scenarioAdoptComponentChild,
			scenarioExtractComponentChild,
			scenarioRenameComponentChild,
			scenarioRetypeComponentChild,
		} {
			if err := scenario(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}
