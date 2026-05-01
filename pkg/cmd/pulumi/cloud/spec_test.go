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

package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndexCarriesRouteProperty asserts the parser surfaces the
// x-pulumi-route-property extension and the native `deprecated` field on
// Operation. The embedded spec is the fixture; we only require at least one
// example of each flavor to prove the path is wired.
func TestIndexCarriesRouteProperty(t *testing.T) {
	t.Parallel()

	idx := loadTestIndex(t)

	var preview, deprecatedExt, superseded int
	for _, op := range idx.Operations {
		if op.IsPreview {
			preview++
		}
		if op.IsDeprecated {
			deprecatedExt++
		}
		if op.SupersededBy != "" {
			superseded++
		}
	}

	assert.Greater(t, preview, 0, "expected at least one preview op in the embedded spec")
	assert.Greater(t, deprecatedExt, 0, "expected at least one deprecated op in the embedded spec")
	assert.Greater(t, superseded, 0, "expected at least one op with SupersededBy in the embedded spec")
}

// TestDeprecatedOpsIncluded guards against the old behavior where deprecated
// operations were silently dropped during parseIndex. Describe must still be
// able to inspect them and ls must be able to opt into showing them.
func TestDeprecatedOpsIncluded(t *testing.T) {
	t.Parallel()

	idx := loadTestIndex(t)

	// /api/ai/template was marked deprecated at the time the spec was pinned.
	op, ok := idx.ByKey["POST /api/ai/template"]
	require.True(t, ok, "deprecated op should appear in the index; was previously silently skipped")
	assert.True(t, op.IsDeprecated, "IsDeprecated should be set from x-pulumi-route-property.Deprecated")
}

// TestInternalOpsFiltered asserts the parser drops any operation whose
// x-pulumi-route-property.Visibility is "Internal". The server only returns
// those to site admins; we still filter defensively so a site admin using
// the CLI sees the same public view everyone else does.
func TestInternalOpsFiltered(t *testing.T) {
	t.Parallel()

	spec := `{
		"openapi": "3.0.0",
		"info": {"title": "t", "version": "0.0.0"},
		"components": {"schemas": {}},
		"paths": {
			"/public": {
				"get": {
					"operationId": "PublicOp",
					"responses": {"200": {"description": "ok"}}
				}
			},
			"/internal": {
				"get": {
					"operationId": "InternalOp",
					"x-pulumi-route-property": {"Visibility": "Internal"},
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`
	idx, err := parseIndex([]byte(spec))
	require.NoError(t, err)
	_, hasPublic := idx.ByKey["GET /public"]
	_, hasInternal := idx.ByKey["GET /internal"]
	assert.True(t, hasPublic, "public op should be indexed")
	assert.False(t, hasInternal, "internal op should be dropped at parse time")
}

// TestSuccessContentTypesCaptured asserts the parser records every content
// type the spec declares on the primary 2xx response — the dispatcher uses
// this list to drive --format-based content negotiation, so dropping
// alternatives at parse time would silently defeat the feature.
func TestSuccessContentTypesCaptured(t *testing.T) {
	t.Parallel()

	spec := `{
		"openapi": "3.0.0",
		"info": {"title": "t", "version": "0.0.0"},
		"components": {"schemas": {}},
		"paths": {
			"/thing": {
				"get": {
					"operationId": "GetThing",
					"responses": {
						"200": {
							"description": "ok",
							"content": {
								"application/json": {"schema": {"type": "object"}},
								"text/markdown": {"schema": {"type": "string"}}
							}
						}
					}
				}
			}
		}
	}`
	idx, err := parseIndex([]byte(spec))
	require.NoError(t, err)
	op, ok := idx.ByKey["GET /thing"]
	require.True(t, ok, "op should be indexed")
	assert.Equal(t, []string{"application/json", "text/markdown"}, op.SuccessContentTypes)
}

// TestSecondTagPreferred verifies the parser picks tags[1] when the spec
// provides a fine-grained sub-grouping. DeploymentRunners is a known sub-tag
// of Workflows in the embedded spec.
func TestSecondTagPreferred(t *testing.T) {
	t.Parallel()

	idx := loadTestIndex(t)

	found := false
	for _, op := range idx.Operations {
		if op.Tag == "DeploymentRunners" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one op tagged DeploymentRunners (second tag of Workflows)")
}
