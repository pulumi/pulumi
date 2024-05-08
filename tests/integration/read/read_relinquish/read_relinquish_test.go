// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
//go:build (nodejs || all) && !xplatform_acceptance

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// Test that the engine is capable of relinquishing control of a resource without deleting it.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestReadRelinquish(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
			},
		},
	})
}
