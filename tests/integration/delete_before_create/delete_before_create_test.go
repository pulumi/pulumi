// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestDeleteBeforeCreate tests a few different operational modes for
// replacements done by deleting before creating.
func TestDeleteBeforeCreate(t *testing.T) {
	t.Parallel()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
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
			{
				Dir:      "step4",
				Additive: true,
			},
			{
				Dir:      "step5",
				Additive: true,
			},
			{
				Dir:      "step6",
				Additive: true,
			},
		},
	})
}
