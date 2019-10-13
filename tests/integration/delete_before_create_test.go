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
		Dir:          "delete_before_create/step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "delete_before_create/step2",
				Additive: true,
			},
			{
				Dir:      "delete_before_create/step3",
				Additive: true,
			},
			{
				Dir:      "delete_before_create/step4",
				Additive: true,
			},
			{
				Dir:      "delete_before_create/step5",
				Additive: true,
			},
			{
				Dir:      "delete_before_create/step6",
				Additive: true,
			},
		},
	})
}
