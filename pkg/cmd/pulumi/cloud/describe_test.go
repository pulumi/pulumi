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
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderDescribeTextShowsPreviewStatus(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method:      "GET",
		Path:        "/api/example",
		OperationID: "GetExample",
		Tag:         "Example",
		IsPreview:   true,
	}
	out := RenderDescribeText(op)
	assert.Contains(t, out, "Status: PREVIEW", "preview ops must surface a status line")
}

func TestRenderDescribeTextShowsDeprecatedStatus(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method:       "GET",
		Path:         "/api/example",
		OperationID:  "GetExample",
		Tag:          "Example",
		IsDeprecated: true,
	}
	out := RenderDescribeText(op)
	assert.Contains(t, out, "Status: DEPRECATED",
		"deprecated ops must surface a status line")
	assert.Contains(t, out, "will be removed in a future version",
		"plain DEPRECATED line should hint at future removal")
}

func TestRenderDescribeTextShowsSupersededBy(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method:       "GET",
		Path:         "/api/example",
		OperationID:  "GetExample",
		Tag:          "Example",
		IsDeprecated: true,
		SupersededBy: "NewExample",
	}
	out := RenderDescribeText(op)
	assert.Contains(t, out, "DEPRECATED — use NewExample instead",
		"SupersededBy should steer users to the replacement op")
	assert.False(t, strings.Contains(out, "will be removed in a future version"),
		"with SupersededBy we use the targeted hint, not the generic one")
}

func TestRenderDescribeTextStableOpsOmitStatus(t *testing.T) {
	t.Parallel()

	op := &Operation{
		Method:      "GET",
		Path:        "/api/example",
		OperationID: "GetExample",
		Tag:         "Example",
	}
	out := RenderDescribeText(op)
	assert.NotContains(t, out, "Status:", "stable ops shouldn't render a Status line")
}

// describeGoldenCases are the operations exercised by the text + markdown
// golden tests. Each case either loads an op from the fixture or builds a
// synthetic *Operation inline so deprecated/preview/superseded variants stay
// deterministic even if the embedded spec changes.
func describeGoldenCases(t *testing.T) []struct {
	name string
	op   *Operation
} {
	t.Helper()
	return []struct {
		name string
		op   *Operation
	}{
		{"list_org_tokens", loadMarkdownOp(t, "ListOrgTokens")},
		{"create_org_token", loadMarkdownOp(t, "CreateOrgToken")},
		{"nested_arrays", loadMarkdownOp(t, "GetNestedArrays")},
		{"preview_op", &Operation{
			Method:      "GET",
			Path:        "/api/preview/things/{id}",
			OperationID: "GetPreviewThing",
			Tag:         "Preview",
			Summary:     "Fetch a preview thing.",
			IsPreview:   true,
			Params: []ParamSpec{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Thing ID."},
			},
		}},
		{"deprecated_with_successor", &Operation{
			Method:       "GET",
			Path:         "/api/old/things",
			OperationID:  "ListOldThings",
			Tag:          "Legacy",
			Summary:      "List legacy things.",
			IsDeprecated: true,
			SupersededBy: "ListThings",
			Params: []ParamSpec{
				{Name: "filter", In: "query", Type: "string", Description: "Optional filter."},
			},
		}},
	}
}

// TestRenderDescribeText_Golden pins the exact text-render output for a set
// of representative operations. Regenerate with PULUMI_ACCEPT=true when the
// render changes intentionally.
func TestRenderDescribeText_Golden(t *testing.T) {
	t.Parallel()
	for _, tc := range describeGoldenCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenderDescribeText(tc.op)
			path := filepath.Join("testdata", "describe_golden", tc.name+".txt")
			assertGolden(t, path, got)
		})
	}
}

// TestRenderDescribeMarkdown_Golden pins the exact markdown-render output.
// Regenerate with PULUMI_ACCEPT=true when the render changes intentionally.
func TestRenderDescribeMarkdown_Golden(t *testing.T) {
	t.Parallel()
	for _, tc := range describeGoldenCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenderDescribeMarkdown(tc.op)
			path := filepath.Join("testdata", "describe_golden", tc.name+".md")
			assertGolden(t, path, got)
		})
	}
}
