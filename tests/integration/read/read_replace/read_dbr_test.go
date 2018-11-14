// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// Test that the engine handles the replacement of an external resource with a
// owned once gracefully.
func TestReadReplace(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"../../../../sdk/nodejs/bin"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
			},
			{
				Dir:      "step3",
				Additive: true,
			},
		},
	})
}
