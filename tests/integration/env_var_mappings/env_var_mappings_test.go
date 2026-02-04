// Copyright 2016-2026, Pulumi Corporation.
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

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnvVarMappings tests that the envVarMappings resource option is correctly
// stored on provider resources.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestEnvVarMappings(t *testing.T) {
	// Set environment variables that would be remapped
	t.Setenv("MY_VAR", "my_value")
	t.Setenv("OTHER_VAR", "other_value")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick:      true,
		NoParallel: true, // Required because we use t.Setenv
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			require.NotNil(t, stackInfo.Deployment)

			// Find the provider resource and verify envVarMappings
			foundProvider := false
			for _, res := range stackInfo.Deployment.Resources {
				if providers.IsProviderType(res.URN.Type()) && res.URN.Name() == "prov" {
					foundProvider = true
					assert.Equal(t, map[string]string{
						"MY_VAR":    "PROVIDER_VAR",
						"OTHER_VAR": "TARGET_VAR",
					}, res.EnvVarMappings, "expected envVarMappings to be set on provider")
					break
				}
			}
			require.True(t, foundProvider, "expected to find provider resource named 'prov'")
		},
	})
}
