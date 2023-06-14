// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build (go || all) && !xplatform_acceptance

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestGoAliases(t *testing.T) {
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
		d := filepath.Join("go", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: filepath.Join(d, "step1"),
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3=../../../sdk",
				},
				Quick: true,
				EditDirs: []integration.EditDir{
					{
						Dir:             filepath.Join(d, "step2"),
						ExpectNoChanges: true,
						Additive:        true,
					},
				},
			})
		})
	}
}

func TestRetypeRemoteComponentAndChild(t *testing.T) {
	dir := filepath.Join("go", "retype_remote_component_and_child")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join(dir, "step1"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3=../../../sdk",
		},
		Quick: true,
		LocalProviders: []integration.LocalDependency{
			{Package: "wibble", Path: filepath.Join(dir, "provider")},
		},
		EditDirs: []integration.EditDir{
			{
				Dir:             filepath.Join(dir, "step2"),
				ExpectNoChanges: true,
				Additive:        true,
			},
		},
	})
}
