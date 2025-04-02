// Copyright 2018-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build (nodejs || all) && !xplatform_acceptance

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// Test that the engine tolerates two deletions of the same URN in the same plan.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDoublePendingDelete(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
		UseNPM:       true,
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:           "step2",
				Additive:      true,
				ExpectFailure: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)

					// Four resources in this deployment: the root resource, A, B, and A (pending delete)
					assert.Equal(t, 5, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stackInfo.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))

					a := stackInfo.Deployment.Resources[2]
					assert.Equal(t, "a", a.URN.Name())
					assert.False(t, a.Delete)

					aCondemned := stackInfo.Deployment.Resources[3]
					assert.Equal(t, "a", aCondemned.URN.Name())
					assert.True(t, aCondemned.Delete)

					b := stackInfo.Deployment.Resources[4]
					assert.Equal(t, "b", b.URN.Name())
					assert.False(t, b.Delete)
				},
			},
			{
				Dir:           "step3",
				Additive:      true,
				ExpectFailure: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// There is still two pending delete resources in this snapshot.
					assert.NotNil(t, stackInfo.Deployment)

					assert.Equal(t, 6, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stackInfo.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))

					a := stackInfo.Deployment.Resources[2]
					assert.Equal(t, "a", a.URN.Name())
					assert.False(t, a.Delete)

					aCondemned := stackInfo.Deployment.Resources[3]
					assert.Equal(t, "a", aCondemned.URN.Name())
					assert.True(t, aCondemned.Delete)

					aSecondCondemned := stackInfo.Deployment.Resources[4]
					assert.Equal(t, "a", aSecondCondemned.URN.Name())
					assert.True(t, aSecondCondemned.Delete)

					b := stackInfo.Deployment.Resources[5]
					assert.Equal(t, "b", b.URN.Name())
					assert.False(t, b.Delete)
				},
			},
			{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// We should have cleared out all of the pending deletes now.
					assert.NotNil(t, stackInfo.Deployment)

					assert.Equal(t, 4, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stackInfo.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))

					a := stackInfo.Deployment.Resources[2]
					assert.Equal(t, "a", a.URN.Name())
					assert.False(t, a.Delete)

					b := stackInfo.Deployment.Resources[3]
					assert.Equal(t, "b", b.URN.Name())
					assert.False(t, b.Delete)
				},
			},
		},
	})
}
