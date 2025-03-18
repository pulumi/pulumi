// Copyright 2016-2024, Pulumi Corporation.
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
	"os"
	"strings"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	atlasAPIPath = "/api/ai/chat/preview"

	// Environment variable to override the Atlas base URL for local development
	debugAtlasBaseVar = "DEBUG_PULUMI_ATLAS_BASE"
)

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

// getSummaryToken retrieves the authentication token for the Atlas API
func getSummaryToken(cloudURL string) (string, error) {
	account, err := workspace.GetAccount(cloudURL)
	if err != nil {
		return "", fmt.Errorf("getting account: %w", err)
	}

	if account.AccessToken == "" {
		return "", fmt.Errorf("no access token found for %s", cloudURL)
	}

	return account.AccessToken, nil
}

// createAtlasRequest creates a new AtlasUpdateSummaryRequest with the given content and org ID
func createAtlasRequest(content string, orgID string) apitype.AtlasUpdateSummaryRequest {
	return apitype.AtlasUpdateSummaryRequest{
		Query: "FIXME in atlas, this is ignored, but still required by zod",
		State: apitype.State{
			Client: apitype.ClientState{
				CloudContext: apitype.CloudContext{
					OrgID: orgID,
					URL:   "https://app.pulumi.com",
				},
			},
		},
		DirectSkillCall: apitype.DirectSkillCall{
			Skill: "summarizeUpdate",
			Params: apitype.SkillParams{
				PulumiUpdateOutput: content,
			},
		},
	}
}

// getAtlasEndpoint returns the configured Atlas endpoint, allowing override via environment variable
func getAtlasEndpoint(cloudURL string) string {
	base := cloudURL
	if debugBase := os.Getenv(debugAtlasBaseVar); debugBase != "" {
		base = debugBase
	}
	return base + atlasAPIPath
}

// summarizeInternal handles the actual summarization logic and returns proper errors
func summarizeInternal(lines []string, orgID string) (string, error) {
	cloudURL, err := getCurrentCloudURL()
	if err != nil {
		return "", fmt.Errorf("getting cloud URL: %w", err)
	}

	token, err := getSummaryToken(cloudURL)
	if err != nil {
		return "", fmt.Errorf("getting authentication token: %w", err)
	}

	content := strings.Join(lines, "\n")

	// Create the request using the helper function
	request := createAtlasRequest(content, orgID)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}

	req, err := http.NewRequest("POST", getAtlasEndpoint(cloudURL), bytes.NewBuffer(jsonData))
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

	var atlasResp apitype.AtlasUpdateSummaryResponse
	if err := json.Unmarshal(body, &atlasResp); err != nil {
		return "", fmt.Errorf("got non-JSON response from Atlas: %s", body)
	}

	if atlasResp.Error != "" {
		return "", fmt.Errorf("atlas API error: %s\n%s", atlasResp.Error, atlasResp.Details)
	}

	// Look for the first summarizeUpdate message
	for _, msg := range atlasResp.ThreadMessages {
		if msg.Kind == "summarizeUpdate" {
			var content apitype.SummarizeUpdate
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				return "", fmt.Errorf("parsing summary content: %w", err)
			}
			if content.Summary == "" {
				return "", errors.New("no summary generated")
			}
			return content.Summary, nil
		}
	}

	return "", errors.New("no summarizeUpdate message found in response")
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
func summarizeErrorWithCopilot(orgID string, lines []string, outputPrefix string) string {
	if len(lines) == 0 {
		return ""
	}

	summary, err := summarizeInternal(lines, orgID)
	if err != nil {
		// TODO: Use proper logging once we have it
		fmt.Fprintf(os.Stderr, "Error generating summary: %v\n", err)
		return ""
	}

	return addPrefixToLines(summary, outputPrefix)
}
