// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// Test that the engine does not consider old inputs when calling Check during re-creation of
// a resource that was deleted due to a dependency on a DBR-replaced resource.
func TestResourceRecreateCheck(t *testing.T) {
	t.Parallel()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "recreate_resource_check/step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "recreate_resource_check/step2",
				Additive: true,
			},
		},
	})
}
