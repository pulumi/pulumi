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

package tests

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertJSONOutput asserts that the named output in props is a JSON string whose parsed value equals want.
// This avoids false failures from equivalent but syntactically different JSON (e.g. key ordering, whitespace).
func assertJSONOutput(l *L, props resource.PropertyMap, key string, secret bool, want any) {
	l.Helper()
	pv, ok := props[resource.PropertyKey(key)]
	if !assert.True(l, ok, "expected output %q to be present", key) {
		return
	}
	if assert.Equal(l, secret, pv.IsSecret(), "expected output %q to not be secret", key) {
		return
	}
	if pv.IsSecret() {
		pv = pv.SecretValue().Element
	}
	if !assert.True(l, pv.IsString(), "expected output %q to be a JSON string, got %v", key, pv) {
		return
	}
	var got any
	require.NoErrorf(l, json.Unmarshal([]byte(pv.StringValue()), &got),
		"output %q is not valid JSON: %q", key, pv.StringValue())
	assert.Equal(l, want, got, "output %q: parsed JSON does not match", key)
}

func init() {
	LanguageTests["l1-builtin-to-json"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-builtin-to-json", "aString"): config.NewValue("hello"),
					config.MustMakeKey("l1-builtin-to-json", "aNumber"): config.NewValue("42"),
					config.MustMakeKey("l1-builtin-to-json", "aList"):   config.NewObjectValue(`["one","two","three"]`),
					//nolint:lll
					config.MustMakeKey("l1-builtin-to-json", "aSecret"): config.NewSecureValue("c2VjcmV0dmFsdWU="), // "secretvalue" in base64
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")

					// Scalar types
					assertJSONOutput(l, outputs, "stringOutput", false, "hello")
					assertJSONOutput(l, outputs, "numberOutput", false, float64(42))
					assertJSONOutput(l, outputs, "boolOutput", false, true)

					// Literal array local
					assertJSONOutput(l, outputs, "arrayOutput", false, []any{"x", "y", "z"})

					// Literal object local
					assertJSONOutput(l, outputs, "objectOutput", false, map[string]any{
						"key":   "value",
						"count": float64(1),
					})

					// Nested object built from config values including secrets, result must remain secret.
					assertJSONOutput(l, outputs, "nestedOutput", true, map[string]any{
						"name":     "hello",
						"items":    []any{"one", "two", "three"},
						"a_secret": "secretvalue",
					})
				},
			},
		},
	}
}
