// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build (python || all) && !xplatform_acceptance

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestPythonAliases(t *testing.T) {
	t.Parallel()

	dirs := []string{
		"rename",
		"adopt_into_component",
		"rename_component_and_child",
		"retype_component",
		"rename_component",
		"retype_parents",
		"adopt_component_child",
		"extract_component_child",
		"rename_component_child",
		"retype_component_child",
	}

	for _, dir := range dirs {
		d := filepath.Join("python", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: filepath.Join(d, "step1"),
				Dependencies: []string{
					filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
				},
				Quick: true,
				EditDirs: []integration.EditDir{
					{
						Dir:             filepath.Join(d, "step2"),
						Additive:        true,
						ExpectNoChanges: true,
					},
				},
			})
		})
	}
}

// TestPythonAliasAfterFailedUpdate is a test for https://github.com/pulumi/pulumi/issues/13848.
func TestPythonAliasAfterFailedUpdate(t *testing.T) {
	t.Parallel()

	d := filepath.Join("python", "alias_after_failed_update")
	t.Run(d, func(t *testing.T) {
		integration.ProgramTest(t, &integration.ProgramTestOptions{
			Dir: filepath.Join(d, "step1"),
			Dependencies: []string{
				filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
			},
			LocalProviders: []integration.LocalDependency{
				{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
			},
			Quick: true,
			EditDirs: []integration.EditDir{
				{
					Dir:           filepath.Join(d, "step2"),
					Additive:      true,
					ExpectFailure: true,
				},
				{
					Dir:      filepath.Join(d, "step3"),
					Additive: true,
				},
			},
		})
	})
}
