// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func validateResources(t *testing.T, resources []apitype.ResourceV3, expectedNames ...string) {
	// Build the lookup table of expected resource names.
	expectedNamesTable := make(map[string]struct{})
	for _, n := range expectedNames {
		expectedNamesTable[n] = struct{}{}
	}

	// Pull out the stack resource, which must be the first resource in the checkpoint.
	stackRes, resources := resources[0], resources[1:]
	assert.Equal(t, resource.RootStackType, stackRes.URN.Type())

	// If there are more resources than just the stack, the second resource will be the default provider.
	if len(resources) > 0 {
		// Pull out the single provider resource, which should be the second resource in the checkpoint.
		providerRes := resources[0]
		resources = resources[1:]
		assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
	}

	// Ensure that the resource count is correct.
	assert.Equal(t, len(resources), len(expectedNames))

	// Ensure that exactly the provided resources are in the array.
	for _, res := range resources {
		name := string(res.URN.Name())
		_, ok := expectedNamesTable[name]
		assert.True(t, ok)
		delete(expectedNamesTable, name)
	}
}

// TestSteps tests many combinations of creates, updates, deletes, replacements, and so on.
func TestSteps(t *testing.T) {
	t.Parallel()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "steps/step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			validateResources(t, stackInfo.Deployment.Resources, "a", "b", "c", "d")
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "steps/step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)
					validateResources(t, stackInfo.Deployment.Resources, "a", "b", "c", "e")
				},
			},
			{
				Dir:      "steps/step3",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)
					validateResources(t, stackInfo.Deployment.Resources, "a", "c", "e")
				},
			},
			{
				Dir:      "steps/step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)
					validateResources(t, stackInfo.Deployment.Resources, "a", "c", "e")
				},
			},
			{
				Dir:      "steps/step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)
					validateResources(t, stackInfo.Deployment.Resources, "a", "c", "e")
				},
			},
			{
				Dir:      "steps/step6",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)
					validateResources(t, stackInfo.Deployment.Resources)
				},
			},
		},
	})
}
