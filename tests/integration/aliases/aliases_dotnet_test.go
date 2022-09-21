// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build (dotnet || all) && !smoke

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

var dirs = []string{
	"rename",
	"adopt_into_component",
	"rename_component_and_child",
	"retype_component",
	"rename_component",
	"retype_parents",
}

func TestDotNetAliases(t *testing.T) {
	t.Parallel()

	for _, dir := range dirs {
		d := filepath.Join("dotnet", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				DebugLogLevel: 9,
				Dir:           filepath.Join(d, "step1"),
				Dependencies:  []string{"Pulumi"},
				Quick:         true,
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
