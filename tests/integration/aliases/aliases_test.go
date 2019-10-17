// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

var dirs = []string{
	"rename",
	"adopt_into_component",
	"rename_component_and_child",
	"retype_component",
	"rename_component",
}

// TestNodejsAliases tests a case where a resource's name changes but it provides an `alias`
// pointing to the old URN to ensure the resource is preserved across the update.
func TestNodejsAliases(t *testing.T) {
	for _, dir := range dirs {
		d := path.Join("nodejs", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:          path.Join(d, "step1"),
				Dependencies: []string{"@pulumi/pulumi"},
				Quick:        true,
				EditDirs: []integration.EditDir{
					{
						Dir:             path.Join(d, "step2"),
						Additive:        true,
						ExpectNoChanges: true,
					},
				},
			})
		})
	}
}

func TestPythonAliases(t *testing.T) {
	for _, dir := range dirs {
		d := path.Join("python", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: path.Join(d, "step1"),
				Dependencies: []string{
					filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
				},
				Quick: true,
				EditDirs: []integration.EditDir{
					{
						Dir:             path.Join(d, "step2"),
						Additive:        true,
						ExpectNoChanges: true,
					},
				},
			})
		})
	}
}
