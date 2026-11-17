// Copyright 2026, Pulumi Corporation.
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

package propertyprint

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       property.Value
		showSecrets bool
		expected    string
	}{
		{
			name:     "null",
			value:    property.Value{},
			expected: "null",
		},
		{
			name:     "bool true",
			value:    property.New(true),
			expected: "true",
		},
		{
			name:     "bool false",
			value:    property.New(false),
			expected: "false",
		},
		{
			name:     "number integer",
			value:    property.New(42.0),
			expected: "42",
		},
		{
			name:     "number float",
			value:    property.New(3.14),
			expected: "3.14",
		},
		{
			name:     "string",
			value:    property.New("hello"),
			expected: `"hello"`,
		},
		{
			name:     "string with quotes",
			value:    property.New(`a "b" c`),
			expected: `"a \"b\" c"`,
		},
		{
			name:     "empty array",
			value:    property.New(property.NewArray(nil)),
			expected: "[]",
		},
		{
			name: "array of primitives",
			value: property.New(property.NewArray([]property.Value{
				property.New(1.0),
				property.New("two"),
				property.New(true),
			})),
			expected: `[1, "two", true]`,
		},
		{
			name:     "empty map",
			value:    property.New(property.NewMap(nil)),
			expected: "{}",
		},
		{
			name: "map with entries (stable order)",
			value: property.New(property.NewMap(map[string]property.Value{
				"b": property.New("two"),
				"a": property.New(1.0),
			})),
			expected: `{"a": 1, "b": "two"}`,
		},
		{
			name: "nested array and map",
			value: property.New(property.NewMap(map[string]property.Value{
				"arr": property.New(property.NewArray([]property.Value{
					property.New(1.0),
					property.New(2.0),
				})),
				"nested": property.New(property.NewMap(map[string]property.Value{
					"k": property.New("v"),
				})),
			})),
			expected: `{"arr": [1, 2], "nested": {"k": "v"}}`,
		},
		{
			name:     "asset with path",
			value:    property.New(&asset.Asset{Path: "/tmp/foo.txt"}),
			expected: `asset(path="/tmp/foo.txt")`,
		},
		{
			name:     "asset with URI",
			value:    property.New(&asset.Asset{URI: "https://example.com/a.txt"}),
			expected: `asset(uri="https://example.com/a.txt")`,
		},
		{
			name:     "archive with path",
			value:    property.New(&archive.Archive{Path: "/tmp/foo.zip"}),
			expected: `archive(path="/tmp/foo.zip")`,
		},
		{
			name: "archive with assets",
			value: property.New(&archive.Archive{Assets: map[string]any{
				"b.txt":      &asset.Asset{Path: "/tmp/b.txt"},
				"a.txt":      &asset.Asset{URI: "https://example.com/a.txt"},
				"nested.zip": &archive.Archive{Path: "/tmp/nested.zip"},
			}}),
			expected: `archive(assets={"a.txt": asset(uri="https://example.com/a.txt"), "b.txt": asset(path="/tmp/b.txt"), "nested.zip": archive(path="/tmp/nested.zip")})`, //nolint:lll
		},
		{
			name:     "archive with URI",
			value:    property.New(&archive.Archive{URI: "https://example.com/a.zip"}),
			expected: `archive(uri="https://example.com/a.zip")`,
		},
		{
			name:        "secret redacted",
			value:       property.New("sensitive").WithSecret(true),
			showSecrets: false,
			expected:    "<secret>",
		},
		{
			name:        "secret shown",
			value:       property.New("sensitive").WithSecret(true),
			showSecrets: true,
			expected:    `"sensitive"`,
		},
		{
			name: "secret inside array redacted",
			value: property.New(property.NewArray([]property.Value{
				property.New("public"),
				property.New("hidden").WithSecret(true),
			})),
			showSecrets: false,
			expected:    `["public", <secret>]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := Value(&buf, tt.value, tt.showSecrets)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}
