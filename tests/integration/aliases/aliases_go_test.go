// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build go all

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestGoAliases(t *testing.T) {
	var dirs = []string{
		"rename",
		"adopt_into_component",
		"rename_component_and_child",
		"retype_component",
		"rename_component",
	}

	for _, dir := range dirs {
		d := filepath.Join("go", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: filepath.Join(d, "step1"),
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3",
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
