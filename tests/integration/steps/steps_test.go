// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestSteps tests many combinations of creates, updates, deletes, replacements, and so on.
func TestSteps(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Checkpoint.Latest)
			assert.Equal(t, 5, len(stackInfo.Checkpoint.Latest.Resources))
			stackRes := stackInfo.Checkpoint.Latest.Resources[0]
			assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
			a := stackInfo.Checkpoint.Latest.Resources[1]
			assert.Equal(t, "a", string(a.URN.Name()))
			b := stackInfo.Checkpoint.Latest.Resources[2]
			assert.Equal(t, "b", string(b.URN.Name()))
			c := stackInfo.Checkpoint.Latest.Resources[3]
			assert.Equal(t, "c", string(c.URN.Name()))
			d := stackInfo.Checkpoint.Latest.Resources[4]
			assert.Equal(t, "d", string(d.URN.Name()))
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 5, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := stackInfo.Checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					b := stackInfo.Checkpoint.Latest.Resources[2]
					assert.Equal(t, "b", string(b.URN.Name()))
					c := stackInfo.Checkpoint.Latest.Resources[3]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stackInfo.Checkpoint.Latest.Resources[4]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			{
				Dir:      "step3",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 4, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := stackInfo.Checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := stackInfo.Checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stackInfo.Checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 4, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := stackInfo.Checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := stackInfo.Checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stackInfo.Checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			{
				Dir:      "step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					// assert.Equal(t, 5, len(checkpoint.Latest.Resources))
					assert.Equal(t, 4, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := stackInfo.Checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := stackInfo.Checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := stackInfo.Checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
					// aPendingDelete := checkpoint.Latest.Resources[4]
					// assert.Equal(t, "a", string(aPendingDelete.URN.Name()))
					// assert.True(t, aPendingDelete.Delete)
				},
			},
			{
				Dir:      "step6",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Checkpoint.Latest)
					assert.Equal(t, 1, len(stackInfo.Checkpoint.Latest.Resources))
					stackRes := stackInfo.Checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				},
			},
		},
	})
}
