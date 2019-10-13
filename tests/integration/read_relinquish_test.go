// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// Test that the engine is capable of relinquishing control of a resource without deleting it.
func TestReadRelinquish(t *testing.T) {
	t.Parallel()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "read/relinquish/step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "read/relinquish/step2",
				Additive: true,
			},
		},
	})
}
