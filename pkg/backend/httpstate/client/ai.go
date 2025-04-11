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
	"errors"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// createSummarizeUpdateRequest creates a new CopilotSummarizeUpdateRequest with the given content and org ID
func createSummarizeUpdateRequest(
	lines []string,
	orgID string,
	model string,
	maxSummaryLen int,
	maxUpdateOutputLen int,
) apitype.CopilotSummarizeUpdateRequest {
	// Convert lines to a single string
	content := strings.Join(lines, "\n")
	content = TruncateWithMiddleOut(content, maxUpdateOutputLen)

	return apitype.CopilotSummarizeUpdateRequest{
		State: apitype.CopilotState{
			Client: apitype.CopilotClientState{
				CloudContext: apitype.CopilotCloudContext{
					OrgID: orgID,
					URL:   "https://app.pulumi.com",
				},
			},
		},
		DirectSkillCall: apitype.CopilotSummarizeUpdate{
			Skill: "summarizeUpdate",
			Params: apitype.CopilotSummarizeUpdateParams{
				PulumiUpdateOutput: content,
				Model:              model,
				MaxLen:             maxSummaryLen,
			},
		},
	}
}

// createSummarizePreviewRequest creates a new CopilotSummarizeUpdateRequest with the given content and org ID
func createSummarizePreviewRequest(
	lines []string,
	orgID string,
	maxUpdateOutputLen int,
) apitype.CopilotSummarizePreviewRequest {
	content := strings.Join(lines, "\n")
	content = TruncateWithMiddleOut(content, maxUpdateOutputLen)

	return apitype.CopilotSummarizePreviewRequest{
		State: apitype.CopilotState{
			Client: apitype.CopilotClientState{
				CloudContext: apitype.CopilotCloudContext{
					OrgID: orgID,
					URL:   "https://app.pulumi.com",
				},
			},
		},
		DirectSkillCall: apitype.CopilotSummarizePreview{
			Skill: "summarizePreview",
			Params: apitype.CopilotSummarizePreviewParams{
				PulumiPreviewOutput: content,
			},
		},
	}
}

// extractCopilotResponse parses the Copilot API response and extracts the summary content
func extractCopilotResponse(copilotResp apitype.CopilotResponse) (string, error) {
	for _, msg := range copilotResp.ThreadMessages {
		if msg.Role != "assistant" {
			continue
		}

		// Handle the new format where content is a string directly
		if msg.Kind == "response" {
			// Unmarshal the RawMessage into a string
			var contentStr string
			if err := json.Unmarshal(msg.Content, &contentStr); err != nil {
				// If it's not a simple string, it might be a raw JSON object Return it as a string representation
				return string(msg.Content), nil
			}
			return contentStr, nil
		}
	}
	return "", errors.New("no assistant message found in response")
}

// Maximum number of characters to send to Copilot. We do this to avoid including a proper token counting library for
// now. Tokens are 3-4 characters as a rough estimate. So this is 1000 tokens.
const maxCopilotContentLength = 4000

const truncationNotice = "... (truncated) ..."

// TruncateWithMiddleOut takes a string and a maximum character count, and returns a new string with content truncated
// from the middle if the total character count exceeds maxChars. This preserves both the beginning and end of the
// content while removing content from the middle.
func TruncateWithMiddleOut(content string, maxChars int) string {
	// If content is shorter than max, return as is
	if len(content) <= maxChars {
		return content
	}

	// If maxChars is too small to even fit truncation notice, just truncate the content directly
	if maxChars <= len(truncationNotice) {
		if maxChars <= 0 {
			return ""
		}
		return content[:maxChars]
	}

	// Calculate how much text we can keep from start and end Subtract truncation notice length and divide remaining
	// space for start/end
	remaining := maxChars - len(truncationNotice)

	startLen := (remaining + 1) / 2
	endLen := remaining / 2

	// Build truncated string with notice in middle
	return content[:startLen] + truncationNotice + content[len(content)-endLen:]
}
