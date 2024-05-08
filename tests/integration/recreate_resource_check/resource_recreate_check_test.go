// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
//go:build (nodejs || all) && !xplatform_acceptance

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// Test that the engine does not consider old inputs when calling Check during re-creation of
// a resource that was deleted due to a dependency on a DBR-replaced resource.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestResourceRecreateCheck(t *testing.T) {
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
