// Copyright 2025-2025, Pulumi Corporation.
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

package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestResourceRowDataColorizedColumns(t *testing.T) {
	t.Parallel()

	out := bytes.Buffer{}
	term := terminal.NewMockTerminal(&out, 80, 20, true)
	_, display := createRendererAndDisplay(term, true)
	display.isTerminal = true

	for _, tt := range []struct {
		name     string
		urn      string
		expected string
	}{
		{
			name:     "control chars",
			urn:      "urn:pulumi:stack:proj::\tprovider:res\n",
			expected: "stack:proj::\\tprovider:res\\n",
		},
		{
			name:     "emoji",
			urn:      "urn:pulumi:stack:proj::provider:🦄",
			expected: "stack:proj::provider:🦄",
		},
		{
			name: "emoji with ZWJ",
			urn:  "urn:pulumi:stack:proj::provider:\U0001F575\U0001F3FD\u200D\u2642\uFE0F", // Emoji with ZWJ 🕵🏽‍♂️
			// Arguably this could be as is without escaping, but
			// strconv.QuoteToGraphic always escales zero width spaces..
			expected: "stack:proj::provider:🕵🏽\\u200d♂️",
		},
		{
			name:     "zwj",
			urn:      "urn:pulumi:stack:proj::provider:A\u00A0Z", // Non-breaking space
			expected: "stack:proj::provider:A\u00a0Z",
		},
		{
			name:     "zwj",
			urn:      "urn:pulumi:stack:proj::provider:A\u200dZ", // ZWJ
			expected: "stack:proj::provider:A\\u200dZ",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := &resourceRowData{
				display:  display,
				diagInfo: &DiagInfo{},
				step: engine.StepEventMetadata{
					URN: resource.URN(tt.urn),
					Op:  deploy.OpUpdate,
				},
				hideRowIfUnnecessary: true,
			}

			cols := row.ColorizedColumns()
			name := cols[nameColumn]
			require.Equal(t, tt.expected, name)
		})
	}
}

// TestGetDiffInfo_FiltersInternalProperties tests that internal properties like __defaults
// are not shown in the short diff display. This is a regression test for issue #2586.
func TestGetDiffInfo_FiltersInternalProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		oldInputs      resource.PropertyMap
		newInputs      resource.PropertyMap
		expectDiff     bool
		shouldMatch    string
		shouldNotMatch string
	}{
		{
			name: "__defaults should be filtered out",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
				"__defaults": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("default1"),
				}),
			},
			expectDiff:     false,
			shouldNotMatch: "__defaults",
		},
		{
			name: "normal property changes should still be shown",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value2"),
			},
			expectDiff:  true,
			shouldMatch: "normalProp",
		},
		{
			name: "both normal and __defaults changes - only normal shown",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value2"),
				"__defaults": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("default1"),
				}),
			},
			expectDiff:     true,
			shouldMatch:    "normalProp",
			shouldNotMatch: "__defaults",
		},
		{
			name: "other internal properties should also be filtered",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
				"__meta":     resource.NewProperty("metadata"),
			},
			expectDiff:     false,
			shouldNotMatch: "__meta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := engine.StepEventMetadata{
				Op: deploy.OpUpdate,
				Old: &engine.StepEventStateMetadata{
					Inputs: tt.oldInputs,
				},
				New: &engine.StepEventStateMetadata{
					Inputs: tt.newInputs,
				},
			}

			result := getDiffInfo(step, apitype.UpdateUpdate)

			if tt.expectDiff {
				require.NotEmpty(t, result, "expected diff output but got none")
			}

			if tt.shouldMatch != "" {
				require.Contains(t, result, tt.shouldMatch,
					"expected diff to contain %q but got: %s", tt.shouldMatch, result)
			}

			if tt.shouldNotMatch != "" {
				require.NotContains(t, result, tt.shouldNotMatch,
					"expected diff to NOT contain %q but got: %s", tt.shouldNotMatch, result)
			}

			// Verify that if there's a diff output, it doesn't contain any internal properties
			if result != "" && strings.Contains(result, "diff:") {
				require.NotContains(t, result, "__",
					"diff output should not contain any internal properties (starting with __): %s", result)
			}
		})
	}
}
