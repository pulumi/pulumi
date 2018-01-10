// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestProtectedResources tests some interesting operations on protected resources.
func TestSteps(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// A single synthetic stack and a single "eternal" resource.
			assert.NotNil(t, stackInfo.Checkpoint.Latest)
			assert.Equal(t, 2, len(stackInfo.Checkpoint.Latest.Resources))
			stackRes := stackInfo.Checkpoint.Latest.Resources[0]
			assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
			a := stackInfo.Checkpoint.Latest.Resources[1]
			assert.Equal(t, "eternal", string(a.URN.Name()))
			assert.True(t, a.Protect)
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// An update to "eternal"; should still be there.
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 2, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := stackInfo.Checkpoint.Latest.Resources[1]
					assert.Equal(t, "eternal", string(a.URN.Name()))
					assert.True(t, a.Protect)
				},
			},
			{
				Dir:      "step3",
				Additive: true,
				// This step will fail because the resource is protected.
				ExpectFailure: true,
			},
			{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// "eternal" should now be unprotected.
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 2, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := stackInfo.Checkpoint.Latest.Resources[1]
					assert.Equal(t, "eternal", string(a.URN.Name()))
					assert.False(t, a.Protect)
				},
			},
			{
				Dir:      "step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// Finally, "eternal" should be deleted.
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 1, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				},
			},
		},
	})
}
