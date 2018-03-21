// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func validateResources(t *testing.T, resources []apitype.Resource, expectedNames ...string) {
	// Build the lookup table of expected resource names.
	expectedNamesTable := make(map[string]struct{})
	for _, n := range expectedNames {
		expectedNamesTable[n] = struct{}{}
	}

	// Ensure that the resource count is correct.
	assert.Equal(t, len(resources), len(expectedNames)+1)

	// Pull out the stack resource, which must be the first resource in the checkpoint.
	stackRes := resources[0]
	assert.Equal(t, resource.RootStackType, stackRes.URN.Type())

	// Ensure that exactly the provided resources are in the array.
	for _, res := range resources[1:] {
		name := string(res.URN.Name())
		_, ok := expectedNamesTable[name]
		assert.True(t, ok)
		delete(expectedNamesTable, name)
	}
}

// TestSteps tests many combinations of creates, updates, deletes, replacements, and so on.
func TestSteps(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Checkpoint.Latest)
			validateResources(t, stackInfo.Checkpoint.Latest.Resources, "a", "b", "c", "d")
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					validateResources(t, stackInfo.Checkpoint.Latest.Resources, "a", "b", "c", "e")
				},
			},
			{
				Dir:      "step3",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					validateResources(t, stackInfo.Checkpoint.Latest.Resources, "a", "c", "e")
				},
			},
			{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					validateResources(t, stackInfo.Checkpoint.Latest.Resources, "a", "c", "e")
				},
			},
			{
				Dir:      "step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					validateResources(t, stackInfo.Checkpoint.Latest.Resources, "a", "c", "e")
				},
			},
			{
				Dir:      "step6",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					validateResources(t, stackInfo.Checkpoint.Latest.Resources)
				},
			},
		},
	})
}
