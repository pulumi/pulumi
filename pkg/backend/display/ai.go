package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var atlasBaseURLs = map[string]string{
	"https://api.pulumi.com": "https://app.pulumi.com",

	// FIXME(dev): Remove before merging
	// Local instance logged into using `pulumi login https://api.simon-local.pulumi.local`
	"https://api.simon-local.pulumi.local": "https://app.simon-local.pulumi.local",
}

const (
	atlasApiPath = "/pulumi-ai/atlas/api/ai/chat/preview"

	// Environment variable to override the Atlas base URL for local development
	debugAtlasBaseVar = "DEBUG_PULUMI_ATLAS_BASE"
)

// Matching the TypeScript interface structure
type SummarizeUpdate struct {
	Summary string `json:"summary"`
}

type Message struct {
	Role    string          `json:"role"`
	Kind    string          `json:"kind"`
	Content json.RawMessage `json:"content"`
}

type AtlasUpdateSummaryResponse struct {
	Messages []Message `json:"messages"`
	Error    string    `json:"error"`
	Details  any       `json:"details"`
}

type CloudContext struct {
	OrgId string `json:"orgId"`
	Url   string `json:"url"`
}

type ClientState struct {
	CloudContext CloudContext `json:"cloudContext"`
}

type State struct {
	Client ClientState `json:"client"`
}

type SkillParams struct {
	PulumiUpdateOutput string `json:"pulumiUpdateOutput"`
}

type DirectSkillCall struct {
	Skill  string      `json:"skill"`
	Params SkillParams `json:"params"`
}

type AtlasUpdateSummaryRequest struct {
	Query           string          `json:"query"`
	State           State           `json:"state"`
	DirectSkillCall DirectSkillCall `json:"directSkillCall"`
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

// getSummaryToken retrieves the authentication token for the Atlas API
func getSummaryToken(cloudUrl string) (string, error) {
	account, err := workspace.GetAccount(cloudUrl)
	if err != nil {
		return "", fmt.Errorf("getting account: %w", err)
	}

	if account.AccessToken == "" {
		return "", fmt.Errorf("no access token found for %s", cloudUrl)
	}

	return account.AccessToken, nil
}

// TODO(atlas): Replace with actual org ID from stack context
// Using pulumi_local as temporary default for development
func getOrgID() (string, error) {
	return "pulumi_local", nil
}

// createAtlasRequest creates a new AtlasUpdateSummaryRequest with the given content and org ID
func createAtlasRequest(content string, orgID string) AtlasUpdateSummaryRequest {
	return AtlasUpdateSummaryRequest{
		Query: "FIXME in atlas, this is ignored, but still required by zod",
		State: State{
			Client: ClientState{
				CloudContext: CloudContext{
					OrgId: orgID,
					Url:   "https://app.pulumi.com",
				},
			},
		},
		DirectSkillCall: DirectSkillCall{
			Skill: "summarizeUpdate",
			Params: SkillParams{
				PulumiUpdateOutput: content,
			},
		},
	}
}

// getAtlasEndpoint returns the configured Atlas endpoint, allowing override via environment variable
func getAtlasEndpoint(cloudUrl string) string {
	base := atlasBaseURLs[cloudUrl]
	if debugBase := os.Getenv(debugAtlasBaseVar); debugBase != "" {
		base = debugBase
	}
	return base + atlasApiPath
}

// summarizeInternal handles the actual summarization logic and returns proper errors
func summarizeInternal(lines []string, orgID string) (string, error) {
	cloudUrl, err := getCurrentCloudURL()
	if err != nil {
		return "", fmt.Errorf("getting cloud URL: %w", err)
	}

	token, err := getSummaryToken(cloudUrl)
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

	req, err := http.NewRequest("POST", getAtlasEndpoint(cloudUrl), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Pulumi-Origin", "app.pulumi.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	var atlasResp AtlasUpdateSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&atlasResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if atlasResp.Error != "" {
		return "", fmt.Errorf("atlas API error: %s\n%s", atlasResp.Error, atlasResp.Details)
	}

	// Look for the first summarizeUpdate message
	for _, msg := range atlasResp.Messages {
		if msg.Kind == "summarizeUpdate" {
			var content SummarizeUpdate
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				return "", fmt.Errorf("parsing summary content: %w", err)
			}
			if content.Summary == "" {
				return "", fmt.Errorf("no summary generated")
			}
			return content.Summary, nil
		}
	}

	return "", fmt.Errorf("no summarizeUpdate message found in response")
}

// summarize generates a summary of the update output
func summarize(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	orgID, err := getOrgID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting org ID: %v\n", err)
		return ""
	}

	summary, err := summarizeInternal(lines, orgID)
	if err != nil {
		// TODO: Use proper logging once we have it
		fmt.Fprintf(os.Stderr, "Error generating summary: %v\n", err)
		return ""
	}
	return summary
}
