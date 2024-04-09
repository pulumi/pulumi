// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build (go || all) && !xplatform_acceptance

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/assert"
)

// TestAnyResource tests the `pulumi.json/#Resource` schema type.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestAnyResource(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			originalValue, ok := stackInfo.Outputs["randResult"].(string)
			assert.True(t, ok)
			echoedValue, ok := stackInfo.Outputs["echoedResult"].(string)
			assert.True(t, ok)
			assert.Equal(t, originalValue, echoedValue)
		},
		Quick: true,
	})
}
