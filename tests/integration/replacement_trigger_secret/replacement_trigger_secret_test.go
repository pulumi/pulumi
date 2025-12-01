// Copyright 2025, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestReplacementTriggerWithSecret(t *testing.T) {
	d := filepath.Join("nodejs", "simple")
	// Use locally built pulumi binary if available
	bin := ""
	relBin := filepath.Join("..", "..", "..", "bin", "pulumi")
	if absBin, err := filepath.Abs(relBin); err == nil {
		if _, err := os.Stat(absBin); err == nil {
			bin = absBin
			t.Logf("Using locally built pulumi binary: %s", bin)
		}
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          d,
		Bin:          bin,
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			randomResName := "testprovider:index:Random"
			var foundRes bool
			for _, res := range stack.Deployment.Resources {
				if res.URN.Name() == "res" {
					foundRes = true
					assert.Equal(t, res.Type, tokens.Type(randomResName))
					require.NotNil(t, res.ReplacementTrigger)

					secret, ok := res.ReplacementTrigger.(map[string]any)
					assert.True(t, ok)
					if ok {
						assert.Equal(t, resource.SecretSig, secret[resource.SigKey])

						_, hasCiphertext := secret["ciphertext"]
						_, hasValue := secret["value"]
						assert.True(t, hasCiphertext || hasValue)
					}
				}
			}
			assert.True(t, foundRes, "expected to find resource 'res'")
		},
	})
}
