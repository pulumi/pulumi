// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build (go || all) && !xplatform_acceptance

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestGoTransformations(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, dir := range Dirs {
		d := filepath.Join("go", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: d,
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3",
				},
				LocalProviders: []integration.LocalDependency{
					{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
				},
				Quick:                  true,
				ExtraRuntimeValidation: Validator,
			})
		})
	}
}
