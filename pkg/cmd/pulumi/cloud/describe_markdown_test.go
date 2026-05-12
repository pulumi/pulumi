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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// loadMarkdownOp resolves an op from the embedded spec for renderer tests.
// Fails the test outright when the op is missing so a spec drift doesn't
// silently mask a bug in the renderer.
func loadMarkdownOp(t *testing.T, id string) *Operation {
	t.Helper()
	idx := loadTestIndex(t)
	res, err := MatchByOperationID(idx, id)
	if err != nil {
		t.Fatalf("match %s: %v", id, err)
	}
	return res.Op
}

func TestRenderDescribeMarkdownHeader(t *testing.T) {
	t.Parallel()

	op := loadMarkdownOp(t, "ListOrgTokens")
	md := RenderDescribeMarkdown(op)

	assert.True(t, strings.HasPrefix(md, "# `GET` /api/orgs/{orgName}/tokens"),
		"first line should be a top-level heading with method chip + path, got: %q",
		firstLine(md))
	assert.Contains(t, md, "**Operation:** `ListOrgTokens`")
	assert.Contains(t, md, "**Tag:** AccessTokens")
}

func TestRenderDescribeMarkdownPreviewAdmonition(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method:      "GET",
		Path:        "/api/preview",
		OperationID: "GetPreview",
		IsPreview:   true,
	}
	md := RenderDescribeMarkdown(op)
	assert.Contains(t, md, "> **Preview**",
		"preview ops should render a blockquote admonition")
}

func TestRenderDescribeMarkdownDeprecatedAdmonition(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method:       "GET",
		Path:         "/api/deprecated",
		OperationID:  "GetDeprecated",
		IsDeprecated: true,
		SupersededBy: "GetReplacement",
	}
	md := RenderDescribeMarkdown(op)
	assert.Contains(t, md, "> **Deprecated**")
	assert.Contains(t, md, "use `GetReplacement` instead")
}

func TestRenderDescribeMarkdownParametersList(t *testing.T) {
	t.Parallel()

	op := loadMarkdownOp(t, "ListOrgTokens")
	md := RenderDescribeMarkdown(op)

	assert.Contains(t, md, "## Parameters")
	// orgName is a required path param.
	assert.Contains(t, md, "- `orgName` `string` _(in: path)_ **required**")
	// filter is an optional query param.
	assert.Contains(t, md, "- `filter` `string` _(in: query)_ _optional_")
}

func TestRenderDescribeMarkdownRequestBody(t *testing.T) {
	t.Parallel()

	op := loadMarkdownOp(t, "CreateOrgToken")
	md := RenderDescribeMarkdown(op)

	assert.Contains(t, md, "## Request body (`application/json`)")
	assert.Contains(t, md, "- `name` `string` **required**",
		"required properties should be flagged")
	assert.Contains(t, md, "- `expires`",
		"optional properties should still appear")
}

func TestRenderDescribeMarkdownResponsesSection(t *testing.T) {
	t.Parallel()

	op := loadMarkdownOp(t, "ListOrgTokens")
	md := RenderDescribeMarkdown(op)

	assert.Contains(t, md, "## Responses")
	assert.Contains(t, md, "### `200 OK`",
		"inline response headings should use the HTTP reason phrase")
}

func TestRenderDescribeMarkdownEnumTail(t *testing.T) {
	t.Parallel()

	vals := markdownEnumTail([]string{"a", "b", "c"})
	assert.Equal(t, "_enum:_ `a`, `b`, `c`", vals)
}

func TestRenderDescribeMarkdownEnumTailTruncates(t *testing.T) {
	t.Parallel()

	vals := markdownEnumTail([]string{"a", "b", "c", "d", "e", "f", "g", "h"})
	assert.Contains(t, vals, "`a`")
	assert.Contains(t, vals, "`f`")
	assert.Contains(t, vals, "…", "more than six enum values should be ellipsized")
	assert.NotContains(t, vals, "`g`", "extra values past 6 are dropped")
}

func TestParametersListEnumTail(t *testing.T) {
	t.Parallel()

	params := []ParamSpec{{
		Name: "level", In: "query", Type: "string",
		Description: "Log level", Values: []string{"info", "warn", "error"},
	}}
	out := parametersList(params)
	assert.Contains(t, out, "- `level` `string` _(in: query)_ _optional_ — Log level")
	assert.Contains(t, out, "_enum:_ `info`, `warn`, `error`")
}

func TestMarkdownStockErrors(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method: "GET", Path: "/api/x", OperationID: "X",
		StockErrors: []ErrorRef{
			{Status: "404", Description: "Not Found"},
			{Status: "429", Description: "Too Many Requests"},
		},
	}
	md := RenderDescribeMarkdown(op)
	assert.Contains(t, md, "**Errors:**")
	assert.Contains(t, md, "`404` (Not Found)")
	assert.Contains(t, md, "`429` (Too Many Requests)")
}

func TestPopulateResponsesSchemalessSuccessIsNotAnError(t *testing.T) {
	t.Parallel()

	// DeletePersonalToken returns 204 No Content; the spec's schemaless 2xx
	// should appear under InlineResponses (rendered as a heading with
	// description), NOT under StockErrors (which is for 4xx/5xx).
	op := loadMarkdownOp(t, "DeletePersonalToken")

	for _, err := range op.StockErrors {
		assert.NotEqual(t, "204", err.Status,
			"204 is a success status and should not be bucketed as a stock error")
	}

	found204 := false
	for _, r := range op.InlineResponses {
		if r.Status == "204" {
			found204 = true
			break
		}
	}
	assert.True(t, found204, "204 should appear under InlineResponses")
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
