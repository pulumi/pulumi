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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
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
