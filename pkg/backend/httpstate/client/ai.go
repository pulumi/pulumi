// Copyright 2016-2025, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// extractSummaryFromResponse parses the Atlas API response and extracts the summary content
func extractSummaryFromResponse(atlasResp apitype.CopilotSummarizeUpdateResponse) (string, error) {
	for _, msg := range atlasResp.ThreadMessages {
		if msg.Role == "assistant" {
			// Handle the new format where content is a string directly
			if msg.Kind == "response" {
				// Unmarshal the RawMessage into a string
				var contentStr string
				if err := json.Unmarshal(msg.Content, &contentStr); err != nil {
					// If it's not a simple string, it might be a raw JSON object
					// Return it as a string representation
					return string(msg.Content), nil
				}
				return contentStr, nil
			}

			// Handle the old format for backward compatibility
			if msg.Kind == "summarizeUpdate" {
				var content apitype.CopilotSummarizeUpdateMessage
				if err := json.Unmarshal(msg.Content, &content); err != nil {
					return "", fmt.Errorf("parsing summary content: %w", err)
				}
				if content.Summary == "" {
					return "", errors.New("no summary generated")
				}
				return content.Summary, nil
			}
		}
	}
	return "", errors.New("no assistant message found in response")
}

// Maximum number of characters to send to Copilot.
// We do this to avoid including a proper token counting library for now.
// Tokens are 3-4 characters as a rough estimate. So this is 1000 tokens.
const maxCopilotContentLength = 4000

const truncationNotice = "... (truncated) ..."

// TruncateWithMiddleOut takes a string and a maximum character count,
// and returns a new string with content truncated from the middle if the total
// character count exceeds maxChars. This preserves both the beginning and end
// of the content while removing content from the middle.
func TruncateWithMiddleOut(content string, maxChars int) string {
	// If content is shorter than max or max is too small, return as is
	if len(content) <= maxChars || maxChars <= len(truncationNotice) {
		return content
	}

	// Calculate how much text we can keep from start and end
	// Subtract truncation notice length and divide remaining space for start/end
	remaining := maxChars - len(truncationNotice)

	startLen := (remaining + 1) / 2
	endLen := remaining / 2

	// Build truncated string with notice in middle
	return content[:startLen] + truncationNotice + content[len(content)-endLen:]
}
