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

package eval

import (
	"testing"

	"github.com/pulumi/esc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyValuePatches(t *testing.T) {
	t.Run("secret object patch", func(t *testing.T) {
		source := []byte(`# example.yaml
values:
  mySecret:
    fn::rotate:
      provider: test
      inputs: {}
      state: null
`)

		// Create a patch with a secret object
		patches := []*Patch{
			{
				DocPath: "values.mySecret[\"fn::rotate\"].state",
				Replacement: esc.NewValue(map[string]esc.Value{
					"credentials": esc.NewSecret(map[string]esc.Value{
						"username": esc.NewValue("admin"),
						"password": esc.NewValue("secret123"),
						"metadata": esc.NewValue(map[string]esc.Value{
							"rotatedAt": esc.NewValue("2024-01-01"),
						}),
					}),
				}),
			},
		}

		result, err := ApplyValuePatches(source, patches)
		assert.NoError(t, err)
		t.Logf("Result YAML:\n%s", string(result))

		// Verify the patched YAML can be parsed without errors
		// to ensure the round-trip works (patch -> YAML -> parse)
		env, diags, err := LoadYAMLBytes("test", result)
		assert.NoError(t, err)
		assert.Empty(t, diags, "patched YAML should parse without errors")
		assert.NotNil(t, env, "should successfully parse environment")
	})
}

func TestValueToSecretJSON(t *testing.T) {
	t.Run("nested secrets", func(t *testing.T) {
		actual, err := valueToSecretJSON(esc.NewValue(map[string]esc.Value{
			"foo": esc.NewValue(map[string]esc.Value{
				"bar": esc.NewSecret("secret"),
			}),
		}))
		require.NoError(t, err)
		expected := map[string]any{
			"foo": map[string]any{
				"bar": map[string]any{
					"fn::secret": "secret",
				},
			},
		}
		assert.Equal(t, expected, actual)
	})

	t.Run("secret object", func(t *testing.T) {
		// When we have a secret object, fn::secret can't handle it directly because it only accepts string literals.
		// Therefore we JSON-encode the object and use fn::fromJSON + fn::secret to decode it.
		actual, err := valueToSecretJSON(esc.NewSecret(map[string]esc.Value{
			"username": esc.NewValue("admin"),
			"password": esc.NewValue("secret123"),
		}))
		require.NoError(t, err)
		expected := map[string]any{
			"fn::fromJSON": map[string]any{
				"fn::secret": `{"password":"secret123","username":"admin"}`,
			},
		}
		assert.Equal(t, expected, actual)
	})
}
