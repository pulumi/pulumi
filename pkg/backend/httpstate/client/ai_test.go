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

package client

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateWithMiddleOut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		maxChars int
		want     string
	}{
		{
			name:     "under limit",
			input:    "short content",
			maxChars: 100,
			want:     "short content",
		},
		{
			name:     "exact limit",
			input:    "12345",
			maxChars: 5,
			want:     "12345",
		},
		{
			name:     "needs truncation",
			input:    "start middle1 middle2 end",
			maxChars: 22,
			want:     "st... (truncated) ...d",
		},
		{
			name:     "single long line",
			input:    "abcdefghijklmnopqrstuvwxyz",
			maxChars: 25,
			want:     "abc... (truncated) ...xyz",
		},
		{
			name:     "empty input",
			input:    "",
			maxChars: 10,
			want:     "",
		},
		// make sure we're handling edge cases where maxChars is less
		// than the truncation notice
		{
			name:     "maxChars less than truncation notice",
			input:    "start middle1 middle2 end",
			maxChars: 6,
			want:     "start ",
		},
		{
			name:     "maxChars is 0",
			input:    "start middle1 middle2 end",
			maxChars: 0,
			want:     "",
		},
		{
			name:     "maxChars is equal to truncation notice",
			input:    "start middle1 middle2 end",
			maxChars: 19,
			want:     "start middle1 middl",
		},
		{
			name:     "maxChars is one longer than truncation notice",
			input:    "start middle1 middle2 end",
			maxChars: 20,
			want:     "s... (truncated) ...",
		},
		{
			name:     "maxChars is two longer than truncation notice",
			input:    "start middle1 middle2 end",
			maxChars: 21,
			want:     "s... (truncated) ...d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateWithMiddleOut(tt.input, tt.maxChars)
			assert.Equal(t, tt.want, got)

			// Verify the result is under the character limit
			if len(tt.input) > 0 {
				totalChars := len(got)
				assert.LessOrEqual(t, totalChars, tt.maxChars, "result exceeds character limit")
			}
		})
	}
}

func TestExtractCopilotResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response apitype.CopilotResponse
		want     string
		wantErr  bool
	}{
		{
			name: "new format - direct string response",
			response: apitype.CopilotResponse{
				ThreadMessages: []apitype.CopilotThreadMessage{
					{
						Role:    "assistant",
						Kind:    "response",
						Content: json.RawMessage(`"This is a summary"`),
					},
				},
			},
			want:    "This is a summary",
			wantErr: false,
		},
		{
			name: "no assistant message",
			response: apitype.CopilotResponse{
				ThreadMessages: []apitype.CopilotThreadMessage{
					{
						Role:    "user",
						Kind:    "response",
						Content: json.RawMessage(`"User message"`),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty summary in old format",
			response: apitype.CopilotResponse{
				ThreadMessages: []apitype.CopilotThreadMessage{
					{
						Role: "assistant",
						Kind: "summarizeUpdate",
						Content: json.RawMessage(`{
							"summary": ""
						}`),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractCopilotResponse(tt.response)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Model and maxLen are optional and the field should be omitted if not provided.
// This is testing some serialization logic too..
func TestCreateSummarizeUpdateRequestOmitsDefaults(t *testing.T) {
	t.Parallel()

	updateRequest := createSummarizeUpdateRequest("line1\nline2\nline3", "org1", "", 0, 100)
	expectedOutput := `{
		"query": "",
		"directSkillCall": {
			"skill": "summarizeUpdate",
			"params": {
				"pulumiUpdateOutput": "line1\nline2\nline3"
			}
		},
		"state": {
			"client": {
				"cloudContext": {
					"orgId": "org1",
					"url": "https://app.pulumi.com"
				}
			}
		}
	}`

	jsonBytes, err := json.Marshal(updateRequest)
	require.NoError(t, err)
	// pretty marshal the expected output
	assert.JSONEq(t, expectedOutput, string(jsonBytes))
}

func TestCreateSummarizeUpdateRequestWithModelAndMaxLen(t *testing.T) {
	t.Parallel()

	updateRequest := createSummarizeUpdateRequest("line1\nline2\nline3", "org1", "gpt-4o-mini", 100, 100)
	expectedOutput := `{
		"query": "",
		"directSkillCall": {
			"skill": "summarizeUpdate",
			"params": {
				"pulumiUpdateOutput": "line1\nline2\nline3",
				"model": "gpt-4o-mini",
				"maxLen": 100
			}
		},
		"state": {
			"client": {
				"cloudContext": {
					"orgId": "org1",
					"url": "https://app.pulumi.com"
				}
			}
		}
	}`

	jsonBytes, err := json.Marshal(updateRequest)
	require.NoError(t, err)
	assert.JSONEq(t, expectedOutput, string(jsonBytes))
}

func TestCreateSummarizeUpdateRequestTruncatesContent(t *testing.T) {
	t.Parallel()

	updateRequest := createSummarizeUpdateRequest("line1\nline2\nline3", "org1", "", 0, 10)
	// The content should be truncated to 10 characters
	assert.Equal(t, "line1\nline", updateRequest.DirectSkillCall.Params.PulumiUpdateOutput)
}
