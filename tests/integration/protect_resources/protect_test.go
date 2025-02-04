// Copyright 2017-2024, Pulumi Corporation.
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

//go:build (nodejs || all)

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// TestProtectedResources tests some interesting operations on protected resources.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestProtectedResources(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// A single synthetic stack and a single "eternal" resource.
			assert.NotNil(t, stackInfo.Deployment)
			assert.Equal(t, 3, len(stackInfo.Deployment.Resources))
			stackRes := stackInfo.Deployment.Resources[0]
			assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
			providerRes := stackInfo.Deployment.Resources[1]
			assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
			a := stackInfo.Deployment.Resources[2]
			assert.Equal(t, "eternal", a.URN.Name())
			assert.True(t, a.Protect)
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// An update to "eternal"; should still be there.
					assert.NotNil(t, stackInfo.Deployment)
					assert.Equal(t, 3, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stackInfo.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
					a := stackInfo.Deployment.Resources[2]
					assert.Equal(t, "eternal", a.URN.Name())
					assert.True(t, a.Protect)
				},
			},
			{
				Dir:      "step3",
				Additive: true,
				// This step will fail because the resource is protected.
				ExpectFailure: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// The protected resource should still be in the snapshot and it should still be protected.
					assert.NotNil(t, stackInfo.Deployment)
					assert.Equal(t, 3, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stackInfo.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
					a := stackInfo.Deployment.Resources[2]
					assert.Equal(t, "eternal", a.URN.Name())
					assert.True(t, a.Protect)
				},
			},
			{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// "eternal" should now be unprotected.
					assert.NotNil(t, stackInfo.Deployment)
					assert.Equal(t, 3, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					providerRes := stackInfo.Deployment.Resources[1]
					assert.True(t, providers.IsProviderType(providerRes.URN.Type()))
					a := stackInfo.Deployment.Resources[2]
					assert.Equal(t, "eternal", a.URN.Name())
					assert.False(t, a.Protect)
				},
			},
			{
				Dir:      "step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// Finally, "eternal" should be deleted.
					assert.NotNil(t, stackInfo.Deployment)
					assert.Equal(t, 1, len(stackInfo.Deployment.Resources))
					stackRes := stackInfo.Deployment.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				},
			},
		},
	})
}
