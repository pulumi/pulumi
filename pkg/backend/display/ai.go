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

package display

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	atlasAPIPath = "/api/ai/chat/preview"

	// Maximum number of characters to send to Copilot.
	// We do this to avoid including a proper token counting library for now.
	// Tokens are 3-4 characters as a rough estimate. So this is 1000 tokens.
	maxCopilotContentLength = 4000
)

// getAtlasEndpoint returns the configured Atlas endpoint
func getAtlasEndpoint(cloudURL string) string {
	return cloudURL + atlasAPIPath
}

func getCurrentCloudURL() (string, error) {
	ws := pkgWorkspace.Instance

	project, _, err := ws.ReadProject()
	if err != nil {
		return "", fmt.Errorf("looking up project: %w", err)
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return "", fmt.Errorf("could not get cloud url: %w", err)
	}

	return url, nil
}

// getAccessToken retrieves the authentication token for the Atlas API
func getAccessToken(cloudURL string) (string, error) {
	account, err := workspace.GetAccount(cloudURL)
	if err != nil {
		return "", fmt.Errorf("getting account: %w", err)
	}

	if account.AccessToken == "" {
		return "", fmt.Errorf("no access token found for %s", cloudURL)
	}

	return account.AccessToken, nil
}

// createSummarizeUpdateRequest creates a new CopilotSummarizeUpdateRequest with the given content and org ID
func createSummarizeUpdateRequest(
	content string,
	orgID string,
	model string,
	maxSummaryLen int,
) apitype.CopilotSummarizeUpdateRequest {
	return apitype.CopilotSummarizeUpdateRequest{
		State: apitype.CopilotState{
			Client: apitype.CopilotClientState{
				CloudContext: apitype.CopilotCloudContext{
					OrgID: orgID,
					URL:   "https://app.pulumi.com",
				},
			},
		},
		DirectSkillCall: apitype.CopilotDirectSkillCall{
			Skill: "summarizeUpdate",
			Params: apitype.CopilotSkillParams{
				PulumiUpdateOutput: content,
				Model:              model,
				MaxLen:             maxSummaryLen,
			},
		},
	}
}

// summarizeInternal handles the actual summarization logic and returns proper errors
func summarizeInternal(content string, orgID string, model string, maxSummaryLen int) (string, error) {
	cloudURL, err := getCurrentCloudURL()
	if err != nil {
		return "", fmt.Errorf("getting cloud URL: %w", err)
	}

	token, err := getAccessToken(cloudURL)
	if err != nil {
		return "", fmt.Errorf("getting authentication token: %w", err)
	}

	// Create the request using the helper function
	request := createSummarizeUpdateRequest(content, orgID, model, maxSummaryLen)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, getAtlasEndpoint(cloudURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Pulumi-Origin", "api.pulumi.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Read the body first so we can use it for error reporting if needed
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}
	resp.Body.Close()

	var atlasResp apitype.CopilotSummarizeUpdateResponse
	if err := json.Unmarshal(body, &atlasResp); err != nil {
		return "", fmt.Errorf("got non-JSON response from Copilot: %s", body)
	}

	if atlasResp.Error != "" {
		return "", fmt.Errorf("atlas API error: %s\n%s", atlasResp.Error, atlasResp.Details)
	}

	return extractSummaryFromResponse(atlasResp)
}

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

// addPrefixToLines adds the given prefix to each line of the input text
func addPrefixToLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// summarize generates a summary of the update output
func summarizeErrorWithCopilot(
	orgID string,
	lines []string,
	outputPrefix string,
	model string,
	maxSummaryLen int,
) (string, error) {
	if len(lines) == 0 {
		return "", nil
	}

	// Convert lines to a single string
	linesStr := strings.Join(lines, "\n")

	linesStr = TruncateWithMiddleOut(linesStr, maxCopilotContentLength)

	startTime := time.Now()

	summary, err := summarizeInternal(linesStr, orgID, model, maxSummaryLen)
	if err != nil {
		return "", err
	}
	elapsedMs := time.Since(startTime).Milliseconds()

	horizontalLine := strings.Repeat("-", 80)
	summaryHeader := fmt.Sprintf("✨ AI-generated summary (took %d ms):", elapsedMs)
	output := fmt.Sprintf("%s\n%s\n\n%s\n", horizontalLine, summaryHeader, summary)

	return addPrefixToLines(output, outputPrefix), nil
}

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
