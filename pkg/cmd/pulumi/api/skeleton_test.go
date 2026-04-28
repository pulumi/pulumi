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

package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skel(t *testing.T, schema string) map[string]any {
	t.Helper()
	out := GenerateBodySkeleton(json.RawMessage(schema))
	require.NotNil(t, out, "skeleton should not be nil for schema=%s", schema)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(out, &parsed))
	return parsed
}

func TestSkeleton_RequiredOnlyWithTypedPlaceholders(t *testing.T) {
	t.Parallel()
	schema := `{
		"type":"object",
		"required":["name","count","enabled"],
		"properties":{
			"name":{"type":"string"},
			"count":{"type":"integer"},
			"enabled":{"type":"boolean"},
			"optional":{"type":"string"}
		}
	}`
	out := skel(t, schema)
	assert.Equal(t, "", out["name"])
	assert.EqualValues(t, 0, out["count"])
	assert.Equal(t, false, out["enabled"])
	_, hasOptional := out["optional"]
	assert.False(t, hasOptional, "optional fields should be omitted")
}

func TestSkeleton_ExamplePreferred(t *testing.T) {
	t.Parallel()
	schema := `{
		"type":"object",
		"required":["stackName"],
		"properties":{"stackName":{"type":"string","example":"dev"}}
	}`
	out := skel(t, schema)
	assert.Equal(t, "dev", out["stackName"])
}

func TestSkeleton_EnumFirstValue(t *testing.T) {
	t.Parallel()
	schema := `{
		"type":"object",
		"required":["role"],
		"properties":{"role":{"type":"string","enum":["admin","member","viewer"]}}
	}`
	out := skel(t, schema)
	assert.Equal(t, "admin", out["role"])
}

func TestSkeleton_FormatPlaceholders(t *testing.T) {
	t.Parallel()
	schema := `{
		"type":"object",
		"required":["id","email","when","home"],
		"properties":{
			"id":{"type":"string","format":"uuid"},
			"email":{"type":"string","format":"email"},
			"when":{"type":"string","format":"date-time"},
			"home":{"type":"string","format":"url"}
		}
	}`
	out := skel(t, schema)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", out["id"])
	assert.Equal(t, "user@example.com", out["email"])
	assert.Equal(t, "2026-01-01T00:00:00Z", out["when"])
	assert.Equal(t, "https://example.com", out["home"])
}

func TestSkeleton_NestedObject(t *testing.T) {
	t.Parallel()
	schema := `{
		"type":"object",
		"required":["owner"],
		"properties":{
			"owner":{
				"type":"object",
				"required":["name","age"],
				"properties":{
					"name":{"type":"string"},
					"age":{"type":"integer"}
				}
			}
		}
	}`
	out := skel(t, schema)
	owner, ok := out["owner"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "", owner["name"])
	assert.EqualValues(t, 0, owner["age"])
}

func TestSkeleton_Array(t *testing.T) {
	t.Parallel()
	schema := `{
		"type":"object",
		"required":["tags"],
		"properties":{
			"tags":{"type":"array","items":{"type":"string"}}
		}
	}`
	out := skel(t, schema)
	arr, ok := out["tags"].([]any)
	require.True(t, ok)
	require.Len(t, arr, 1)
	assert.Equal(t, "", arr[0])
}

func TestSkeleton_AllOfMergesRequired(t *testing.T) {
	t.Parallel()
	schema := `{
		"allOf":[
			{"type":"object","required":["a"],"properties":{"a":{"type":"string"}}},
			{"type":"object","required":["b"],"properties":{"b":{"type":"integer"}}}
		]
	}`
	out := skel(t, schema)
	assert.Equal(t, "", out["a"])
	assert.EqualValues(t, 0, out["b"])
}

func TestSkeleton_OneOfPicksFirstBranch(t *testing.T) {
	t.Parallel()
	schema := `{
		"oneOf":[
			{
				"type":"object","required":["kind","subject"],
				"properties":{"kind":{"type":"string","enum":["user"]},"subject":{"type":"string"}}
			},
			{
				"type":"object","required":["kind","team"],
				"properties":{"kind":{"type":"string","enum":["team"]},"team":{"type":"string"}}
			}
		]
	}`
	out := skel(t, schema)
	assert.Equal(t, "user", out["kind"])
	assert.Equal(t, "", out["subject"])
	_, hasTeam := out["team"]
	assert.False(t, hasTeam)
}

func TestSkeleton_EmptySchemaReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, GenerateBodySkeleton(nil))
	assert.Nil(t, GenerateBodySkeleton([]byte{}))
}

func TestSkeleton_MalformedInputReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, GenerateBodySkeleton([]byte("not json")))
}

// TestSkeleton_RealCreateStack pins the skeleton for CreateStack against the
// embedded spec so regressions in either the spec sync or the generator are
// caught. `stackName` is the only required field; `tags`/`teams`/`config`
// are optional and should be omitted.
func TestSkeleton_RealCreateStack(t *testing.T) {
	t.Parallel()
	idx := loadTestIndex(t)
	op, ok := idx.ByKey["POST /api/stacks/{orgName}/{projectName}"]
	require.True(t, ok, "CreateStack must exist in the embedded spec")
	require.NotNil(t, op.BodySchemaJSON)

	out := skel(t, string(op.BodySchemaJSON))
	assert.Equal(t, "", out["stackName"])
	for _, optional := range []string{"tags", "teams", "config", "state"} {
		_, present := out[optional]
		assert.False(t, present, "optional field %q should be omitted from skeleton", optional)
	}
}
