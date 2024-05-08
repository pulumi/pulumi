// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build (nodejs || all) && !xplatform_acceptance

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsSimpleTransforms(t *testing.T) {
	d := filepath.Join("nodejs", "simple")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          d,
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick:                  true,
		ExtraRuntimeValidation: Validator,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsSingleTransforms(t *testing.T) {
	d := filepath.Join("nodejs", "single")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          d,
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
	})
}
