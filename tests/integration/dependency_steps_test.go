// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestDependencySteps tests a case where the dependency graph between two
// resources is inverted between updates. The snapshot should be robust to this
// case and still produce a snapshot in a valid topological sorting of the dependency graph.
func TestDependencySteps(t *testing.T) {
	t.Parallel()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "dependency_steps/step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "dependency_steps/step2",
				Additive: true,
			},
		},
	})
}
