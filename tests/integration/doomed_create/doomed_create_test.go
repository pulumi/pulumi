// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

// TestDoomedCreation is a negative test that asserts that we are able to recover
// gracefully from a failed resource creation.
func TestDoomedCreation(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           "step1",
		Dependencies:  []string{"@pulumi/pulumi"},
		Quick:         true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			if !assert.NotNil(t, stack.Deployment) {
				t.FailNow()
			}

			latest := stack.Deployment
			// There should be one resource in the snapshot:
			//  resource "a", in the "creating" state (NOT created)
			assert.Len(t, latest.Resources, 1)
			res := latest.Resources[0]
			assert.Equal(t, resource.ResourceStatusCreating, res.Status)
		},
		EditDirs: []integration.EditDir{
			// This edit fixes the resource error. The creation will be successful.
			// Also tests that the engine does not attempt to delete the "creating" resource
			{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					if !assert.NotNil(t, stack.Deployment) {
						t.FailNow()
					}

					latest := stack.Deployment
					// There should be one resource in the snapshot:
					//  resource "a", in the "created" state
					assert.Len(t, latest.Resources, 1)
					res := latest.Resources[0]
					assert.Equal(t, resource.ResourceStatusCreated, res.Status)
				},
			},
		},
	})
}
