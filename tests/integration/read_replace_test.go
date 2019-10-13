// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// Test that the engine handles the replacement of an external resource with a
// owned once gracefully.
func TestReadReplace(t *testing.T) {
	t.Parallel()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "read/replace/step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "read/replace/step2",
				Additive: true,
			},
			{
				Dir:      "read/replace/step3",
				Additive: true,
			},
		},
	})
}
