// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestAliases tests a case where a resource's name changes but it provides an `alias` pointing to the old URN to ensure
// the resource is preserved across the update.
func TestAliases(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:             "step2",
				Additive:        true,
				ExpectNoChanges: true,
			},
		},
	})
}
