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
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// RenderDescribeMarkdown produces a markdown document describing the operation,
// laid out to mirror pulumi/docs' OpenAPI rendering (method + path header,
// admonitions for preview/deprecated, description, a parameters table, request
// body, and a bucketed list of responses). The output is plain GitHub-flavored
// markdown suitable for `describe --format=markdown`; callers piping to a
// terminal may further pass it through glow or similar for ANSI rendering.
func RenderDescribeMarkdown(op *Operation) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# `%s` %s\n\n", op.Method, op.Path)

	summary := strings.TrimSpace(op.Summary)
	if summary == "" {
		summary = op.OperationID
	}
	if summary != "" {
		fmt.Fprintf(&b, "_%s_\n\n", summary)
	}
	if op.OperationID != "" {
		fmt.Fprintf(&b, "**Operation:** `%s`", op.OperationID)
		if op.Tag != "" {
			fmt.Fprintf(&b, " · **Tag:** %s", op.Tag)
		}
		b.WriteString("\n\n")
	}

	if op.IsPreview {
		b.WriteString("> **Preview** — this endpoint is not yet stable and may change without notice.\n\n")
	}
	if op.IsDeprecated {
		if op.SupersededBy != "" {
			fmt.Fprintf(&b, "> **Deprecated** — use `%s` instead.\n\n", op.SupersededBy)
		} else {
			b.WriteString("> **Deprecated** — will be removed in a future version.\n\n")
		}
	}

	if desc := strings.TrimSpace(op.Description); desc != "" && desc != strings.TrimSpace(op.Summary) {
		b.WriteString("## Description\n\n")
		b.WriteString(desc)
		b.WriteString("\n\n")
	}

	if len(op.Params) > 0 {
		b.WriteString("## Parameters\n\n")
		b.WriteString(parametersList(op.Params))
		b.WriteString("\n")
	}

	if op.HasBody {
		if op.BodyContentType != "" {
			fmt.Fprintf(&b, "## Request body (`%s`)\n\n", op.BodyContentType)
		} else {
			b.WriteString("## Request body\n\n")
		}
		if op.BodySchemaMarkdown != "" {
			b.WriteString(op.BodySchemaMarkdown)
		} else {
			b.WriteString("_No schema._")
		}
		b.WriteString("\n\n")
	}

	if len(op.InlineResponses) > 0 {
		b.WriteString("## Responses\n\n")
		for _, r := range op.InlineResponses {
			reason := httpReasonPhrase(r.Status)
			status := r.Status
			if reason != "" {
				status = r.Status + " " + reason
			}
			// Elide the description when it duplicates the reason phrase (e.g.
			// spec description "No Content" on a 204).
			desc := strings.TrimSpace(r.Description)
			if desc == "" || strings.EqualFold(desc, reason) {
				fmt.Fprintf(&b, "### `%s`\n\n", status)
			} else {
				fmt.Fprintf(&b, "### `%s` — %s\n\n", status, desc)
			}
			if r.SchemaMarkdown != "" {
				b.WriteString(r.SchemaMarkdown)
				b.WriteString("\n\n")
			}
		}
	}
	if len(op.StockErrors) > 0 {
		b.WriteString("**Errors:** ")
		parts := make([]string, 0, len(op.StockErrors))
		for _, e := range op.StockErrors {
			if e.Description != "" {
				parts = append(parts, fmt.Sprintf("`%s` (%s)", e.Status, e.Description))
			} else {
				parts = append(parts, fmt.Sprintf("`%s`", e.Status))
			}
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// parametersList renders parameters as a bullet list. Each row carries the
// same data a table would — name, location, required flag, type, and a
// flattened description — but without GFM table formatting so terminal
// renderers that style tables with backgrounds don't clobber the rest of the
// page. Mirrors pulumi/docs' `parameters.html` partial.
func parametersList(params []ParamSpec) string {
	rows := append([]ParamSpec(nil), params...)
	inPrecedence := map[string]int{"path": 0, "query": 1, "header": 2, "cookie": 3}
	sort.SliceStable(rows, func(i, j int) bool {
		pi, pj := inPrecedence[rows[i].In], inPrecedence[rows[j].In]
		if pi != pj {
			return pi < pj
		}
		return rows[i].Name < rows[j].Name
	})

	var b strings.Builder
	for _, p := range rows {
		req := "_optional_"
		if p.Required {
			req = "**required**"
		}
		fmt.Fprintf(&b, "- `%s` `%s` _(in: %s)_ %s", p.Name, p.Type, p.In, req)
		if desc := mdInline(p.Description); desc != "" {
			fmt.Fprintf(&b, " — %s", desc)
		}
		b.WriteByte('\n')
		if len(p.Values) > 0 {
			fmt.Fprintf(&b, "  - %s\n", markdownEnumTail(p.Values))
		}
	}
	return b.String()
}

// httpReasonPhrase returns the standard HTTP reason phrase for a numeric
// status code ("OK" for 200, "Not Found" for 404). Returns "" for non-numeric
// or unrecognized status strings so callers can fall back to the bare code.
func httpReasonPhrase(status string) string {
	n, err := strconv.Atoi(status)
	if err != nil {
		return ""
	}
	return http.StatusText(n)
}
