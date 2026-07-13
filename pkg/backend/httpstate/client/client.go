// Copyright 2016, Pulumi Corporation.
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
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/bits"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/opentracing/opentracing-go"
	"go.opentelemetry.io/otel"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util/validation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	// 20s before we give up on a warm, interactive Neo request.
	NeoRequestTimeout = 20 * time.Second
	// Task creation can trigger a backend cold start (up to ~1 min after a new
	// Neo version is deployed), so allow it a much longer budget than warm requests.
	NeoCreateTaskTimeout = 120 * time.Second
)

// NeoApprovalMode controls whether the agent requires user approval before executing tools.
// Mirrors apitype.NeoApprovalMode in pulumi-service; the wire values must stay in sync.
type NeoApprovalMode string

const (
	// NeoApprovalModeManual requires the agent to request user approval for each tool call.
	NeoApprovalModeManual NeoApprovalMode = "manual"
	// NeoApprovalModeBalanced auto-approves low-risk tool calls and prompts only on
	// destructive operations. The cloud ApprovalHandler decides which calls qualify.
	NeoApprovalModeBalanced NeoApprovalMode = "balanced"
	// NeoApprovalModeAuto allows the agent to execute tools without user approval.
	NeoApprovalModeAuto NeoApprovalMode = "auto"
)

// NeoTaskSource identifies the origin that triggered a Neo task.
type NeoTaskSource string

const (
	// NeoTaskSourceCLI tags tasks created from the Pulumi CLI.
	NeoTaskSourceCLI NeoTaskSource = "cli"
)

// NeoPermissionMode caps the capabilities granted to an agent task. Mirrors
// apitype.NeoPermissionMode in pulumi-service; the wire values must stay in sync.
type NeoPermissionMode string

const (
	// NeoPermissionModeDefault grants the agent the full set of capabilities permitted
	// by the user's role. This is the server's default when the field is omitted.
	NeoPermissionModeDefault NeoPermissionMode = "default"
	// NeoPermissionModeReadOnly restricts the agent to read-only operations: no
	// `pulumi up`, no PR creation, no state mutations.
	NeoPermissionModeReadOnly NeoPermissionMode = "read-only"
)

// NeoTaskRequest represents a request to create a Neo task. This is a thin client-side
// shape that the server deserializes into apitype.CreateAgentTaskRequest, so the JSON
// field names must match the IDL-generated tags exactly.
type NeoTaskRequest struct {
	Message NeoTaskMessage `json:"message"`
	// ToolExecutionMode selects where Neo tool calls run. Empty (the default) and "cloud"
	// mean tools run in the agent container as before; "cli" means the cloud agent emits
	// cli_tool_request backend events for the local-tool subset (filesystem, shell,
	// pulumi_preview, pulumi_up) and waits for cli_tool_result user events in response.
	// JSON tag is camelCase to match apitype.CreateAgentTaskRequest from pulumi-service.
	ToolExecutionMode string `json:"toolExecutionMode,omitempty"`
	// ApprovalMode controls whether the agent requires user approval before executing tools.
	// JSON tag is camelCase to match apitype.CreateAgentTaskRequest from pulumi-service.
	ApprovalMode NeoApprovalMode `json:"approvalMode,omitempty"`
	// PermissionMode caps the agent's capabilities (default vs read-only). Empty means
	// inherit the org / server default. JSON tag is camelCase to match the service IDL.
	PermissionMode NeoPermissionMode `json:"permissionMode,omitempty"`
	// PlanMode, when true, creates the task in plan mode: the agent explores and asks
	// questions but must not write files, run `pulumi up`, or open PRs. The server enforces
	// this by activating PlanModeTracker for the task and gating the exit on an approved
	// exit_plan_mode call. JSON tag is camelCase to match the service IDL.
	PlanMode bool `json:"planMode,omitempty"`
	// Source identifies the origin that triggered the task. The CLI always sends
	// NeoTaskSourceCLI; the server validates against apitype.AgentTaskSource and defaults
	// to "api" if omitted.
	Source NeoTaskSource `json:"source,omitempty"`
	// EnabledIntegrations is a three-state pointer matching apitype.CreateAgentTaskRequest:
	// nil inherits all org-enabled integrations, a non-nil empty slice sends `[]` to opt out
	// of every integration, and a populated slice allow-lists specific ones.
	EnabledIntegrations *[]string `json:"enabledIntegrations,omitempty"`
}

// NeoTaskMessage represents the message content for a Neo task.
type NeoTaskMessage struct {
	Type       string             `json:"type"`
	Content    string             `json:"content"`
	Timestamp  string             `json:"timestamp"`
	EntityDiff *NeoTaskEntityDiff `json:"entity_diff,omitempty"`
}

// AgentSignupChallenge is returned by the unauthenticated agent signup
// challenge endpoint.
type AgentSignupChallenge struct {
	ChallengeID   string `json:"challengeID"`
	ChallengeData string `json:"challengeData"`
}

// AgentSignupResponse is returned after solving an unauthenticated agent signup
// challenge.
type AgentSignupResponse struct {
	AccessToken           string    `json:"accessToken"`
	AccessTokenValidUntil time.Time `json:"accessTokenValidUntil"`
	RefreshToken          string    `json:"refreshToken,omitempty"`
	ClaimToken            string    `json:"claimToken"`
	ClaimTokenValidUntil  time.Time `json:"claimTokenValidUntil"`
}

// agentSignupRequest is sent to the unauthenticated agent signup endpoint with
// the solved challenge and best-effort agent metadata.
type agentSignupRequest struct {
	ChallengeID              string `json:"challengeID,omitempty"`
	ChallengeResult          string `json:"challengeResult,omitempty"`
	AgentName                string `json:"agentName,omitempty"`
	AgentModel               string `json:"agentModel,omitempty"`
	ChallengeSolveDurationMS int64  `json:"challengeSolveDurationMs,omitempty"`
}

// NeoTaskEntityDiff represents entities to add or remove from the agent context.
type NeoTaskEntityDiff struct {
	Add    []NeoTaskEntity `json:"add,omitempty"`
	Remove []NeoTaskEntity `json:"remove,omitempty"`
}

// NeoTaskEntity represents an entity (like a stack) that the agent can work with.
type NeoTaskEntity struct {
	// Type can be "stack", "repository", "pull_request" or "policy_issue"
	Type    string `json:"type"`
	Name    string `json:"name"`
	Project string `json:"project"`
}

// NeoTaskResponse represents the response from creating a Neo task.
type NeoTaskResponse struct {
	TaskID string `json:"taskId"`
}

// NeoTask represents the fields from an existing Neo task that the CLI needs
// when reattaching to it.
type NeoTask struct {
	TaskID         string            `json:"taskId"`
	ApprovalMode   NeoApprovalMode   `json:"approvalMode,omitempty"`
	PermissionMode NeoPermissionMode `json:"permissionMode,omitempty"`
}

// TemplatePublishOperationID uniquely identifies a template publish operation.
type TemplatePublishOperationID string

// StartTemplatePublishRequest is the request body for starting a template publish operation.
type StartTemplatePublishRequest struct {
	// Version is the semantic version of the template.
	Version semver.Version `json:"version"`
}

// StartTemplatePublishResponse is the response from initiating a template publish.
// It returns a presigned URL to upload the template archive.
type StartTemplatePublishResponse struct {
	// OperationID uniquely identifies the publishing operation.
	OperationID TemplatePublishOperationID `json:"operationID"`
	// UploadURLs contains the presigned URLs for uploading template artifacts.
	UploadURLs TemplateUploadURLs `json:"uploadURLs"`
}

// TemplateUploadURLs contains the presigned URLs for uploading template artifacts.
type TemplateUploadURLs struct {
	// Archive is the URL for uploading the template archive.
	Archive string `json:"archive"`
}

// PublishTemplateVersionCompleteRequest is the request body for completing a template publish operation.
type PublishTemplateVersionCompleteRequest struct {
	// OpID is the operation ID from the StartTemplatePublishResponse.
	OpID TemplatePublishOperationID `json:"operationID"`
}

// PublishTemplateVersionCompleteResponse is the response from completing a template publish operation.
type PublishTemplateVersionCompleteResponse struct{}

// Client provides a slim wrapper around the Pulumi HTTP/REST API.
type Client struct {
	apiURL     string
	apiToken   accessToken
	apiUser    string
	apiOrgs    []string
	tokenInfo  *workspace.TokenInformation // might be nil if running against old services
	diag       diag.Sink
	insecure   bool
	restClient restClient

	// If true, do not probe the backend with GET /api/capabilities and assume no capabilities.
	DisableCapabilityProbing bool
}

// newClient creates a new Pulumi API client with the given URL and API token. It is a variable instead of a regular
// function so it can be set to a different implementation at runtime, if necessary.
var newClient = func(apiURL, apiToken string, insecure bool, d diag.Sink) *Client {
	var httpClient *http.Client
	if insecure {
		tr := &http.Transport{
			//nolint:gosec // The user has explicitly opted into setting this
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient = &http.Client{Transport: tr}
	} else {
		httpClient = http.DefaultClient
	}

	return &Client{
		apiURL:   apiURL,
		apiToken: apiAccessToken(apiToken),
		diag:     d,
		restClient: &defaultRESTClient{
			client: &defaultHTTPClient{
				client: httpClient,
			},
		},
	}
}

// Returns true if this client is insecure (i.e. has TLS disabled).
func (pc *Client) Insecure() bool {
	return pc.insecure
}

// NewClient creates a new Pulumi API client with the given URL and API token.
func NewClient(apiURL, apiToken string, insecure bool, d diag.Sink) *Client {
	return newClient(apiURL, apiToken, insecure, d)
}

// WithHTTPClient sets the HTTP client for the API client.
// Useful for testing.
func (pc *Client) WithHTTPClient(httpClient *http.Client) *Client {
	pc.restClient = &defaultRESTClient{
		client: &defaultHTTPClient{
			client: httpClient,
		},
	}
	return pc
}

// WithRefresh wires an OAuth refresh token + a credentials-writeback callback into this client.
// Once configured, the client transparently exchanges the refresh token at /api/oauth/token for a
// fresh access token whenever the service rejects the current one with 401, retrying the original
// request before falling through to LoginRequiredError. Passing an empty refresh token is a no-op
// so callers can guard on workspace.Account.RefreshToken without a separate branch.
func (pc *Client) WithRefresh(
	refreshToken string,
	writeback func(accessToken string, accessTokenExpiresAt time.Time, refreshToken string) error,
) *Client {
	if refreshToken == "" {
		return pc
	}
	contract.Requiref(writeback != nil, "writeback", "must not be nil when refreshToken is non-empty")
	initial, _ := pc.apiToken.Get(context.Background())
	pc.apiToken = &refreshableAPIAccessToken{
		accessToken:  initial,
		refreshToken: refreshToken,
		refresh: func(ctx context.Context, rt string) (string, time.Time, string, error) {
			resp, err := pc.RefreshAccessToken(ctx, rt)
			if err != nil {
				return "", time.Time{}, "", err
			}
			var expiresAt time.Time
			if resp.ExpiresIn > 0 {
				expiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
			}
			return resp.AccessToken, expiresAt, resp.RefreshToken, nil
		},
		writeback: writeback,
	}
	return pc
}

// URL returns the URL of the API endpoint this client interacts with
func (pc *Client) URL() string {
	return pc.apiURL
}

// SignupAgent creates an ephemeral account for the detected agent using the
// unauthenticated signup endpoint.
func (pc *Client) SignupAgent(ctx context.Context, metadata agentdetect.Metadata) (AgentSignupResponse, error) {
	var challenge AgentSignupChallenge
	if err := pc.restCall(ctx, http.MethodGet, "/api/agents/signup", nil, nil, &challenge); err != nil {
		return AgentSignupResponse{}, err
	}
	if challenge.ChallengeID == "" || challenge.ChallengeData == "" {
		return AgentSignupResponse{}, errors.New(
			"creating agent Pulumi account: signup response did not include challenge data")
	}

	challengeStart := time.Now()
	challengeResult, err := solveAgentSignupChallenge(ctx, challenge.ChallengeData)
	if err != nil {
		return AgentSignupResponse{}, err
	}
	challengeSolveDuration := time.Since(challengeStart)

	var resp AgentSignupResponse
	if err := pc.restCall(ctx, http.MethodPost, "/api/agents/signup", nil, agentSignupRequest{
		ChallengeID:              challenge.ChallengeID,
		ChallengeResult:          challengeResult,
		AgentName:                metadata.Name,
		AgentModel:               metadata.Model,
		ChallengeSolveDurationMS: challengeSolveDuration.Milliseconds(),
	}, &resp); err != nil {
		return AgentSignupResponse{}, err
	}
	if resp.AccessToken == "" {
		return AgentSignupResponse{}, errors.New(
			"creating agent Pulumi account: signup response did not include an access token")
	}
	if resp.AccessTokenValidUntil.IsZero() {
		return AgentSignupResponse{}, errors.New(
			"creating agent Pulumi account: signup response did not include accessTokenValidUntil")
	}
	if resp.ClaimToken == "" {
		return AgentSignupResponse{}, errors.New(
			"creating agent Pulumi account: signup response did not include a claim token")
	}
	if resp.ClaimTokenValidUntil.IsZero() {
		return AgentSignupResponse{}, errors.New(
			"creating agent Pulumi account: signup response did not include claimTokenValidUntil")
	}
	return resp, nil
}

// ValidateAgentClaim reports whether an agent claim token is still claimable.
// It uses the unauthenticated signup validation endpoint.
func (pc *Client) ValidateAgentClaim(ctx context.Context, claimToken string) (bool, error) {
	if strings.TrimSpace(claimToken) == "" {
		return false, nil
	}
	err := pc.restCall(ctx, http.MethodGet, "/api/agents/signup/validate/"+url.PathEscape(claimToken), nil, nil, nil)
	if err == nil {
		return true, nil
	}
	var errResp *apitype.ErrorResponse
	if errors.As(err, &errResp) && errResp.Code == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

// solveAgentSignupChallenge finds a nonce satisfying the signup proof-of-work
// challenge and returns it as the challenge result.
func solveAgentSignupChallenge(ctx context.Context, data string) (string, error) {
	difficulty, err := parseAgentSignupChallengeDifficulty(data)
	if err != nil {
		return "", err
	}
	for nonce := uint64(0); ; nonce++ {
		if nonce%4096 == 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
		}
		result := strconv.FormatUint(nonce, 10)
		hash := sha256.Sum256([]byte(data + ":" + result))
		if leadingZeroBits(hash[:]) >= difficulty {
			return result, nil
		}
		if nonce == ^uint64(0) {
			return "", errors.New("creating agent Pulumi account: exhausted challenge nonce space")
		}
	}
}

// parseAgentSignupChallengeDifficulty extracts the proof-of-work difficulty
// from versioned challenge data.
func parseAgentSignupChallengeDifficulty(data string) (int, error) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "v1" {
		return 0, errors.New("creating agent Pulumi account: invalid challenge data")
	}
	difficulty, err := strconv.Atoi(parts[2])
	if err != nil || difficulty <= 0 || difficulty > 256 {
		return 0, errors.New("creating agent Pulumi account: invalid challenge difficulty")
	}
	return difficulty, nil
}

// leadingZeroBits returns the number of leading zero bits in a byte slice.
func leadingZeroBits(b []byte) int {
	n := 0
	for _, x := range b {
		if x == 0 {
			n += 8
			continue
		}
		n += bits.LeadingZeros8(x)
		break
	}
	return n
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCall(ctx context.Context, method, path string, queryObj, reqObj, respObj any) error {
	return pc.restClient.Call(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken,
		httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCallWithOptions(
	ctx context.Context, method, path string, queryObj, reqObj,
	respObj any, opts httpCallOptions,
) error {
	return pc.restClient.Call(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, opts)
}

// updateRESTCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. The call is authorized with the indicated update token. If a response object is provided, the server's
// response is deserialized into that object.
func (pc *Client) updateRESTCall(ctx context.Context, method, path string, queryObj, reqObj, respObj any,
	token updateToken, httpOptions httpCallOptions,
) error {
	return pc.restClient.Call(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, token, httpOptions)
}

// getProjectPath returns the API path for the given owner and the given project name joined with path separators
// and appended to the stack root.
func getProjectPath(owner string, projectName string) string {
	return fmt.Sprintf("/api/stacks/%s/%s", owner, projectName)
}

// getStackPath returns the API path to for the given stack with the given components joined with path separators
// and appended to the stack root.
func getStackPath(stack StackIdentifier, components ...string) string {
	prefix := fmt.Sprintf("/api/stacks/%s/%s/%s", stack.Owner, stack.Project, stack.Stack)
	return path.Join(append([]string{prefix}, components...)...)
}

// listPolicyGroupsPath returns the path for an API call to the Pulumi service to list the Policy Groups
// in a Pulumi organization.
func listPolicyGroupsPath(orgName string) string {
	return fmt.Sprintf("/api/orgs/%s/policygroups", orgName)
}

// listPolicyPacksPath returns the path for an API call to the Pulumi service to list the Policy Packs
// in a Pulumi organization.
func listPolicyPacksPath(orgName string) string {
	return fmt.Sprintf("/api/orgs/%s/policypacks", orgName)
}

// publishPolicyPackPath returns the path for an API call to the Pulumi service to publish a new Policy Pack
// in a Pulumi organization.
func publishPolicyPackPath(orgName string) string {
	return fmt.Sprintf("/api/orgs/%s/policypacks", orgName)
}

// updatePolicyGroupPath returns the path for an API call to the Pulumi service to update a PolicyGroup
// for a Pulumi organization.
func updatePolicyGroupPath(orgName, policyGroup string) string {
	return fmt.Sprintf(
		"/api/orgs/%s/policygroups/%s", orgName, policyGroup)
}

// deletePolicyPackPath returns the path for an API call to the Pulumi service to delete
// all versions of a Policy Pack from a Pulumi organization.
func deletePolicyPackPath(orgName, policyPackName string) string {
	return fmt.Sprintf("/api/orgs/%s/policypacks/%s", orgName, policyPackName)
}

// deletePolicyPackVersionPath returns the path for an API call to the Pulumi service to delete
// a version of a Policy Pack from a Pulumi organization.
func deletePolicyPackVersionPath(orgName, policyPackName, versionTag string) string {
	return fmt.Sprintf(
		"/api/orgs/%s/policypacks/%s/versions/%s", orgName, policyPackName, versionTag)
}

// publishPolicyPackPublishComplete returns the path for an API call to signal to the Pulumi service
// that a PolicyPack to a Pulumi organization.
func publishPolicyPackPublishComplete(orgName, policyPackName string, versionTag string) string {
	return fmt.Sprintf(
		"/api/orgs/%s/policypacks/%s/versions/%s/complete", orgName, policyPackName, versionTag)
}

// getPolicyPackConfigSchemaPath returns the API path to retrieve the policy pack configuration schema.
func getPolicyPackConfigSchemaPath(orgName, policyPackName string, versionTag string) string {
	return fmt.Sprintf(
		"/api/orgs/%s/policypacks/%s/versions/%s/schema", orgName, policyPackName, versionTag)
}

// getAIPromptPath returns the API path to create a Pulumi AI prompt.
func getAIPromptPath() string {
	return "/api/ai/template"
}

// getUpdatePath returns the API path to for the given stack with the given components joined with path separators
// and appended to the update root.
func getUpdatePath(update UpdateIdentifier, components ...string) string {
	components = append([]string{string(apitype.UpdateUpdate), update.UpdateID}, components...)
	return getStackPath(update.StackIdentifier, components...)
}

func publishPackagePath(source, publisher, name string) string {
	return fmt.Sprintf("/api/registry/packages/%s/%s/%s/versions", source, publisher, name)
}

func completePackagePublishPath(source, publisher, name, version string) string {
	return fmt.Sprintf("/api/registry/packages/%s/%s/%s/versions/%s/complete", source, publisher, name, version)
}

func deletePackageVersionPath(source, publisher, name, version string) string {
	return fmt.Sprintf("/api/registry/packages/%s/%s/%s/versions/%s", source, publisher, name, version)
}

func publishTemplatePath(source, publisher, name string) string {
	return fmt.Sprintf("/api/registry/templates/%s/%s/%s/versions", source, publisher, name)
}

func completeTemplatePublishPath(source, publisher, name string, version semver.Version) string {
	return fmt.Sprintf("/api/registry/templates/%s/%s/%s/versions/%s/complete", source, publisher, name, version)
}

// Copied from https://github.com/pulumi/pulumi-service/blob/master/pkg/apitype/users.go#L7-L16
type serviceUserInfo struct {
	Name        string `json:"name"`
	GitHubLogin string `json:"githubLogin"`
	AvatarURL   string `json:"avatarUrl"`
	Email       string `json:"email,omitempty"`
}

// Copied from https://github.com/pulumi/pulumi-service/blob/master/pkg/apitype/users.go#L20-L37
type serviceUser struct {
	ID            string            `json:"id"`
	GitHubLogin   string            `json:"githubLogin"`
	Name          string            `json:"name"`
	Email         string            `json:"email"`
	AvatarURL     string            `json:"avatarUrl"`
	Organizations []serviceUserInfo `json:"organizations"`
	Identities    []string          `json:"identities"`
	SiteAdmin     *bool             `json:"siteAdmin,omitempty"`
	TokenInfo     *serviceTokenInfo `json:"tokenInfo,omitempty"`
}

// Copied from https://github.com/pulumi/pulumi-service/blob/master/pkg/apitype/users.go#L39-L43
type serviceTokenInfo struct {
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	Team         string `json:"team,omitempty"`
}

//nolint:gosec
const (
	pulumiAccessTokenTypeOrganization = "urn:pulumi:token-type:access_token:organization"
	pulumiAccessTokenTypeTeam         = "urn:pulumi:token-type:access_token:team"
	pulumiAccessTokenTypePersonal     = "urn:pulumi:token-type:access_token:personal"
)

func (pc *Client) ExchangeOidcToken(
	ctx context.Context,
	oidcToken string,
	org string,
	scope string,
	expiration time.Duration,
) (*apitype.TokenExchangeGrantResponse, error) {
	requestedTokenType := pulumiAccessTokenTypeOrganization
	if strings.HasPrefix(scope, "team:") {
		requestedTokenType = pulumiAccessTokenTypeTeam
	}
	if strings.HasPrefix(scope, "user:") {
		requestedTokenType = pulumiAccessTokenTypePersonal
	}
	tokenURL := pc.apiURL + "/api/oauth/token"
	data := url.Values{
		"audience":             {"urn:pulumi:org:" + org},
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"requested_token_type": {requestedTokenType},
		"scope":                {scope},
		"subject_token_type":   {"urn:ietf:params:oauth:token-type:id_token"},
		"subject_token":        {oidcToken},
		"expiration":           {strconv.Itoa(int(expiration.Seconds()))},
	}
	bodyReader := strings.NewReader(data.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := pc.restClient.HTTPClient().Do(req, retryAllMethods)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", string(body))
	}
	var unmarshalledResp apitype.TokenExchangeGrantResponse
	err = json.Unmarshal(body, &unmarshalledResp)
	if err != nil {
		return nil, err
	}
	return &unmarshalledResp, nil
}

// RefreshAccessToken exchanges a Pulumi-issued refresh token for a fresh access token via
// /api/oauth/token (grant_type=refresh_token, RFC 6749 §6). Returns the parsed token response;
// the response's RefreshToken is the value to use on subsequent calls (the server may or may
// not rotate it). The caller is responsible for writing the response's AccessToken back into
// credentials.json when the exchange succeeds.
func (pc *Client) RefreshAccessToken(
	ctx context.Context,
	refreshToken string,
) (*apitype.TokenExchangeGrantResponse, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is required")
	}
	tokenURL := pc.apiURL + "/api/oauth/token"
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	bodyReader := strings.NewReader(data.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := pc.restClient.HTTPClient().Do(req, retryAllMethods)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// Forward the server's RFC 6749 §5.2 error payload as the error message so callers can
		// distinguish invalid_grant (token gone / revoked / wrong type) from unsupported_grant_type
		// (LD kill switch flipped) and react accordingly.
		return nil, fmt.Errorf("refresh_token grant failed: %s: %s", resp.Status, string(body))
	}
	var unmarshalledResp apitype.TokenExchangeGrantResponse
	if err := json.Unmarshal(body, &unmarshalledResp); err != nil {
		return nil, err
	}
	if unmarshalledResp.AccessToken == "" {
		return nil, errors.New("refresh_token grant returned empty access_token")
	}
	return &unmarshalledResp, nil
}

// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
func (pc *Client) GetPulumiAccountDetails(ctx context.Context) (string, []string, *workspace.TokenInformation, error) {
	if pc.apiUser == "" {
		resp := serviceUser{}
		if err := pc.restCall(ctx, "GET", "/api/user", nil, nil, &resp); err != nil {
			return "", nil, nil, err
		}

		if resp.GitHubLogin == "" {
			return "", nil, nil, errors.New("unexpected response from server")
		}

		pc.apiUser = resp.GitHubLogin
		pc.apiOrgs = make([]string, len(resp.Organizations))
		for i, org := range resp.Organizations {
			if org.GitHubLogin == "" {
				return "", nil, nil, errors.New("unexpected response from server")
			}

			pc.apiOrgs[i] = org.GitHubLogin
		}
		if resp.TokenInfo != nil {
			pc.tokenInfo = &workspace.TokenInformation{
				Name:         resp.TokenInfo.Name,
				Organization: resp.TokenInfo.Organization,
				Team:         resp.TokenInfo.Team,
			}
		}
	}

	return pc.apiUser, pc.apiOrgs, pc.tokenInfo, nil
}

// GetCLIVersionInfo asks the service for information about versions of the CLI (the newest version as well as the
// oldest version before the CLI should warn about an upgrade, and the current dev version).
func (pc *Client) GetCLIVersionInfo(
	ctx context.Context,
	metadata map[string]string,
) (semver.Version, semver.Version, semver.Version, error) {
	var versionInfo apitype.CLIVersionResponse

	headers := map[string][]string{}
	for k, v := range metadata {
		headers["X-Pulumi-"+k] = []string{v}
	}

	err := pc.restCallWithOptions(
		ctx,
		"GET",
		"/api/cli/version",
		nil,          // query
		nil,          // request
		&versionInfo, // response
		httpCallOptions{
			RetryPolicy: retryNone,
			Header:      http.Header(headers),
		},
	)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	latestSem, err := semver.ParseTolerant(versionInfo.LatestVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	oldestSem, err := semver.ParseTolerant(versionInfo.OldestWithoutWarning)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	// If there is no dev version, return the latest and oldest
	// versions.  This can happen if the server does not include
	// https://github.com/pulumi/pulumi-service/pull/17429 yet
	if versionInfo.LatestDevVersion == "" {
		return latestSem, oldestSem, semver.Version{}, nil
	}

	devSem, err := semver.ParseTolerant(versionInfo.LatestDevVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	return latestSem, oldestSem, devSem, nil
}

// GetDefaultOrg lists the backend's opinion of which user organization to use, if default organization
// is unset. This API should only be called if the backend supports DefaultOrg as a capability.
func (pc *Client) GetDefaultOrg(ctx context.Context) (apitype.GetDefaultOrganizationResponse, error) {
	var resp apitype.GetDefaultOrganizationResponse
	if err := pc.restCall(ctx, "GET", "/api/user/organizations/default", nil, nil, &resp); err != nil {
		if is404(err) {
			// The client continues to support legacy backends. They do not support GetDefaultOrg; decision
			// for default org is left for the CLI to determine.
			return apitype.GetDefaultOrganizationResponse{}, nil
		}
		return apitype.GetDefaultOrganizationResponse{}, err
	}
	return resp, nil
}

// ListStacksFilter describes optional filters when listing stacks.
type ListStacksFilter struct {
	Project      *string
	Organization *string
	TagName      *string
	TagValue     *string
}

// ListStacks lists all stacks the current user has access to, optionally filtered by project.
func (pc *Client) ListStacks(
	ctx context.Context, filter ListStacksFilter, inContToken *string,
) ([]apitype.StackSummary, *string, error) {
	queryFilter := struct {
		Project           *string `url:"project,omitempty"`
		Organization      *string `url:"organization,omitempty"`
		TagName           *string `url:"tagName,omitempty"`
		TagValue          *string `url:"tagValue,omitempty"`
		ContinuationToken *string `url:"continuationToken,omitempty"`
	}{
		Project:           filter.Project,
		Organization:      filter.Organization,
		TagName:           filter.TagName,
		TagValue:          filter.TagValue,
		ContinuationToken: inContToken,
	}

	var resp apitype.ListStacksResponse
	if err := pc.restCall(ctx, "GET", "/api/user/stacks", queryFilter, nil, &resp); err != nil {
		return nil, nil, err
	}

	return resp.Stacks, resp.ContinuationToken, nil
}

// ListOrgTemplates lists the project templates associated with an org.
func (pc *Client) ListOrgTemplates(ctx context.Context, org string) (apitype.ListOrgTemplatesResponse, error) {
	var resp apitype.ListOrgTemplatesResponse
	if err := pc.restCall(ctx, "GET", "/api/orgs/"+url.PathEscape(org)+"/templates", nil, nil, &resp); err != nil {
		return apitype.ListOrgTemplatesResponse{}, err
	}

	return resp, nil
}

// A [tar.Reader] that owns it's underlying data, and is thus responsible for closing it.
type TarReaderCloser struct {
	data io.ReadCloser
}

func (trc *TarReaderCloser) Tar() *tar.Reader { return tar.NewReader(trc.data) }

func (trc *TarReaderCloser) Close() error { return trc.data.Close() }

func (pc *Client) DownloadOrgTemplate(ctx context.Context, org, sourceURL string) (*TarReaderCloser, error) {
	path := "/api/orgs/" + url.PathEscape(org) + "/template/download?url=" + url.PathEscape(sourceURL)

	header := make(http.Header, 1)
	header.Add("Accept", "application/x-tar")

	var resp io.ReadCloser
	if err := pc.restCallWithOptions(ctx, "GET", path, nil, nil, &resp, httpCallOptions{
		Header: header,
	}); err != nil {
		return nil, err
	}
	return &TarReaderCloser{data: resp}, nil
}

// ErrNoPreviousDeployment is returned when there isn't a previous deployment.
var ErrNoPreviousDeployment = errors.New("no previous deployment")

type getLatestConfigurationResponse struct {
	Info apitype.UpdateInfo `json:"info,omitempty"`
}

type LatestConfiguration struct {
	Config       config.Map // The stack config
	Environments []string   // The environments the stack is configured with
}

// GetLatestConfiguration returns the configuration for the latest deployment of a given stack.
func (pc *Client) GetLatestConfiguration(ctx context.Context, stackID StackIdentifier) (LatestConfiguration, error) {
	var latest getLatestConfigurationResponse
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "updates", "latest"), nil, nil, &latest); err != nil {
		if restErr, ok := err.(*apitype.ErrorResponse); ok {
			if restErr.Code == http.StatusNotFound {
				return LatestConfiguration{}, ErrNoPreviousDeployment
			}
		}

		return LatestConfiguration{}, err
	}

	cfg := make(config.Map, len(latest.Info.Config))
	for k, v := range latest.Info.Config {
		newKey, err := config.ParseKey(k)
		if err != nil {
			return LatestConfiguration{}, err
		}
		if v.Object {
			if v.Secret {
				cfg[newKey] = config.NewSecureObjectValue(v.String)
			} else {
				cfg[newKey] = config.NewObjectValue(v.String)
			}
		} else {
			if v.Secret {
				cfg[newKey] = config.NewSecureValue(v.String)
			} else {
				cfg[newKey] = config.NewValue(v.String)
			}
		}
	}

	const stackEnvironments = "stack.environments"

	var environments []string
	if envs, ok := latest.Info.Environment[stackEnvironments]; ok {
		var parsedEnvs []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(envs), &parsedEnvs); err != nil {
			return LatestConfiguration{}, err
		}
		environments = make([]string, len(parsedEnvs))
		for i, v := range parsedEnvs {
			if v.ID == "" {
				return LatestConfiguration{}, fmt.Errorf(`%s[%d] missing "id" property`, stackEnvironments, i)
			}
			environments[i] = v.ID
		}
	}

	return LatestConfiguration{Config: cfg, Environments: environments}, nil
}

// DoesProjectExist returns true if a project with the given name exists, or false otherwise.
func (pc *Client) DoesProjectExist(ctx context.Context, owner string, projectName string) (bool, error) {
	if err := pc.restCall(ctx, "HEAD", getProjectPath(owner, projectName), nil, nil, nil); err != nil {
		// If this was a 404, return false - project not found.
		if is404(err) {
			return false, nil
		}

		return false, err
	}
	return true, nil
}

// GetStack retrieves the stack with the given name.
func (pc *Client) GetStack(ctx context.Context, stackID StackIdentifier) (apitype.Stack, error) {
	var stack apitype.Stack
	if err := pc.restCall(ctx, "GET", getStackPath(stackID), nil, nil, &stack); err != nil {
		return apitype.Stack{}, err
	}
	return stack, nil
}

// ListDriftRuns returns a paginated list of drift detection runs for the given stack.
func (pc *Client) ListDriftRuns(
	ctx context.Context, stackID StackIdentifier, page, pageSize int,
) (apitype.ListDriftRunsResponse, error) {
	queryObj := struct {
		Page     int `url:"page"`
		PageSize int `url:"pageSize"`
	}{
		Page:     page,
		PageSize: pageSize,
	}
	var resp apitype.ListDriftRunsResponse
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "drift", "runs"), queryObj, nil, &resp); err != nil {
		return apitype.ListDriftRunsResponse{}, err
	}
	return resp, nil
}

// ListOrgWebhooks returns all webhooks configured for the given organization.
func (pc *Client) ListOrgWebhooks(ctx context.Context, org string) ([]apitype.Webhook, error) {
	var resp []apitype.Webhook
	if err := pc.restCall(ctx, "GET", "/api/orgs/"+url.PathEscape(org)+"/hooks", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateOrgWebhook creates a new webhook for the given organization.
func (pc *Client) CreateOrgWebhook(
	ctx context.Context, org string, req apitype.Webhook,
) (apitype.Webhook, error) {
	var resp apitype.Webhook
	if err := pc.restCall(ctx, "POST", "/api/orgs/"+url.PathEscape(org)+"/hooks", nil, &req, &resp); err != nil {
		return apitype.Webhook{}, err
	}
	return resp, nil
}

// GetOrgWebhook returns a single webhook by name for the given organization.
func (pc *Client) GetOrgWebhook(ctx context.Context, org, webhookName string) (apitype.Webhook, error) {
	var resp apitype.Webhook
	path := "/api/orgs/" + url.PathEscape(org) + "/hooks/" + url.PathEscape(webhookName)
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.Webhook{}, err
	}
	return resp, nil
}

// UpdateOrgWebhook updates an existing webhook for the given organization.
func (pc *Client) UpdateOrgWebhook(
	ctx context.Context, org, webhookName string, req apitype.Webhook,
) (apitype.Webhook, error) {
	var resp apitype.Webhook
	path := "/api/orgs/" + url.PathEscape(org) + "/hooks/" + url.PathEscape(webhookName)
	if err := pc.restCall(ctx, "PATCH", path, nil, &req, &resp); err != nil {
		return apitype.Webhook{}, err
	}
	return resp, nil
}

// DeleteOrgWebhook deletes the given webhook from the organization.
func (pc *Client) DeleteOrgWebhook(ctx context.Context, org, webhookName string) error {
	return pc.restCall(ctx, "DELETE",
		"/api/orgs/"+url.PathEscape(org)+"/hooks/"+url.PathEscape(webhookName), nil, nil, nil)
}

// PingOrgWebhook sends a test ping to the given organization webhook.
func (pc *Client) PingOrgWebhook(
	ctx context.Context, org, webhookName string,
) (apitype.WebhookDelivery, error) {
	var resp apitype.WebhookDelivery
	path := "/api/orgs/" + url.PathEscape(org) + "/hooks/" + url.PathEscape(webhookName) + "/ping"
	if err := pc.restCall(ctx, "POST", path, nil, nil, &resp); err != nil {
		return apitype.WebhookDelivery{}, err
	}
	return resp, nil
}

// ListOrgWebhookDeliveries returns recent deliveries for the given org webhook.
func (pc *Client) ListOrgWebhookDeliveries(
	ctx context.Context, org, webhookName string,
) ([]apitype.WebhookDelivery, error) {
	var resp []apitype.WebhookDelivery
	path := "/api/orgs/" + url.PathEscape(org) + "/hooks/" + url.PathEscape(webhookName) + "/deliveries"
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListStackWebhooks returns all webhooks configured for the given stack.
func (pc *Client) ListStackWebhooks(ctx context.Context, stackID StackIdentifier) ([]apitype.Webhook, error) {
	var resp []apitype.Webhook
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "hooks"), nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStackWebhook returns a single webhook by name for the given stack.
func (pc *Client) GetStackWebhook(
	ctx context.Context, stackID StackIdentifier, webhookName string,
) (apitype.Webhook, error) {
	var resp apitype.Webhook
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "hooks", webhookName), nil, nil, &resp); err != nil {
		return apitype.Webhook{}, err
	}
	return resp, nil
}

// PingStackWebhook sends a test ping to the given webhook and returns the delivery result.
func (pc *Client) PingStackWebhook(
	ctx context.Context, stackID StackIdentifier, webhookName string,
) (apitype.WebhookDelivery, error) {
	var resp apitype.WebhookDelivery
	if err := pc.restCall(ctx, "POST", getStackPath(stackID, "hooks", webhookName, "ping"), nil, nil, &resp); err != nil {
		return apitype.WebhookDelivery{}, err
	}
	return resp, nil
}

// CreateStackWebhook creates a new webhook for the given stack.
func (pc *Client) CreateStackWebhook(
	ctx context.Context, stackID StackIdentifier, req apitype.Webhook,
) (apitype.Webhook, error) {
	var resp apitype.Webhook
	if err := pc.restCall(ctx, "POST", getStackPath(stackID, "hooks"), nil, &req, &resp); err != nil {
		return apitype.Webhook{}, err
	}
	return resp, nil
}

// GetDriftStatus returns the current drift detection status for the given stack.
func (pc *Client) GetDriftStatus(
	ctx context.Context, stackID StackIdentifier,
) (apitype.StackDriftStatus, error) {
	var resp apitype.StackDriftStatus
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "drift", "status"), nil, nil, &resp); err != nil {
		return apitype.StackDriftStatus{}, err
	}
	return resp, nil
}

// UpdateStackWebhook updates an existing webhook for the given stack.
func (pc *Client) UpdateStackWebhook(
	ctx context.Context, stackID StackIdentifier, webhookName string, req apitype.Webhook,
) (apitype.Webhook, error) {
	var resp apitype.Webhook
	if err := pc.restCall(ctx, "PATCH", getStackPath(stackID, "hooks", webhookName), nil, &req, &resp); err != nil {
		return apitype.Webhook{}, err
	}
	return resp, nil
}

// DeleteStackWebhook deletes the given webhook from the stack.
func (pc *Client) DeleteStackWebhook(
	ctx context.Context, stackID StackIdentifier, webhookName string,
) error {
	return pc.restCall(ctx, "DELETE", getStackPath(stackID, "hooks", webhookName), nil, nil, nil)
}

// ListStackSchedules returns all scheduled deployment actions configured for the given stack.
func (pc *Client) ListStackSchedules(
	ctx context.Context, stackID StackIdentifier,
) ([]apitype.ScheduledAction, error) {
	var resp apitype.ListScheduledActionsResponse
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "deployments", "schedules"), nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Schedules, nil
}

// GetStackSchedule returns the scheduled deployment action with the given ID.
func (pc *Client) GetStackSchedule(
	ctx context.Context, stackID StackIdentifier, scheduleID string,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "schedules", scheduleID)
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// CreateStackSchedule creates a custom scheduled deployment action for the given stack.
// Exactly one of req.ScheduleCron or req.ScheduleOnce must be set. The stack must have
// deployment settings configured before a schedule can be created.
func (pc *Client) CreateStackSchedule(
	ctx context.Context, stackID StackIdentifier, req apitype.CreateScheduledDeploymentRequest,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "schedules")
	if err := pc.restCall(ctx, "POST", path, nil, req, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// CreateStackDriftSchedule creates a scheduled drift-detection action for the given stack.
// The stack must have deployment settings configured before a schedule can be created.
func (pc *Client) CreateStackDriftSchedule(
	ctx context.Context, stackID StackIdentifier, req apitype.CreateScheduledDriftDeploymentRequest,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "drift", "schedules")
	if err := pc.restCall(ctx, "POST", path, nil, req, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// CreateStackTTLSchedule creates a scheduled TTL (one-time destroy) action for the given stack.
// The stack must have deployment settings configured before a schedule can be created.
func (pc *Client) CreateStackTTLSchedule(
	ctx context.Context, stackID StackIdentifier, req apitype.CreateScheduledTTLDeploymentRequest,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "ttl", "schedules")
	if err := pc.restCall(ctx, "POST", path, nil, req, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// DeleteStackSchedule permanently deletes a scheduled deployment action.
func (pc *Client) DeleteStackSchedule(
	ctx context.Context, stackID StackIdentifier, scheduleID string,
) error {
	path := getStackPath(stackID, "deployments", "schedules", scheduleID)
	return pc.restCall(ctx, "DELETE", path, nil, nil, nil)
}

// UpdateStackSchedule updates a raw scheduled deployment action. The full request body is expected: callers should read
// the current schedule and pass back any fields they want to preserve (the service treats omitted bool options as
// false).
func (pc *Client) UpdateStackSchedule(
	ctx context.Context, stackID StackIdentifier, scheduleID string,
	req apitype.CreateScheduledDeploymentRequest,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "schedules", scheduleID)
	if err := pc.restCall(ctx, "POST", path, nil, req, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// UpdateStackDriftSchedule updates a drift-detection scheduled deployment action. The full request body is expected:
// callers should read the current schedule and pass back any fields they want to preserve (the service treats omitted
// bool options as false).
func (pc *Client) UpdateStackDriftSchedule(
	ctx context.Context, stackID StackIdentifier, scheduleID string,
	req apitype.CreateScheduledDriftDeploymentRequest,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "drift", "schedules", scheduleID)
	if err := pc.restCall(ctx, "POST", path, nil, req, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// UpdateStackTTLSchedule updates a TTL scheduled deployment action. The full request body is expected: callers should
// read the current schedule and pass back any fields they want to preserve (the service treats omitted bool options as
// false).
func (pc *Client) UpdateStackTTLSchedule(
	ctx context.Context, stackID StackIdentifier, scheduleID string,
	req apitype.CreateScheduledTTLDeploymentRequest,
) (apitype.ScheduledAction, error) {
	var resp apitype.ScheduledAction
	path := getStackPath(stackID, "deployments", "ttl", "schedules", scheduleID)
	if err := pc.restCall(ctx, "POST", path, nil, req, &resp); err != nil {
		return apitype.ScheduledAction{}, err
	}
	return resp, nil
}

// ListStackWebhookDeliveries returns recent deliveries for the given webhook.
func (pc *Client) ListStackWebhookDeliveries(
	ctx context.Context, stackID StackIdentifier, webhookName string,
) ([]apitype.WebhookDelivery, error) {
	var resp []apitype.WebhookDelivery
	path := getStackPath(stackID, "hooks", webhookName, "deliveries")
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RedeliverStackWebhookEvent triggers redelivery of a specific event to the given webhook.
func (pc *Client) RedeliverStackWebhookEvent(
	ctx context.Context, stackID StackIdentifier, webhookName, eventID string,
) (apitype.WebhookDelivery, error) {
	var resp apitype.WebhookDelivery
	path := getStackPath(stackID, "hooks", webhookName, "deliveries", eventID, "redeliver")
	if err := pc.restCall(ctx, "POST", path, nil, nil, &resp); err != nil {
		return apitype.WebhookDelivery{}, err
	}
	return resp, nil
}

// CreateStackDetails holds additional information returned by the Pulumi Service when a stack is
// created, beyond the stack itself.
type CreateStackDetails struct {
	Messages []apitype.Message
}

// CreateStack creates a stack with the given cloud and stack name in the scope of the indicated
// project. It returns the created stack along with any side information the backend wants the CLI
// to act on (e.g. messages to display to the user).
func (pc *Client) CreateStack(
	ctx context.Context,
	stackID StackIdentifier,
	tags map[apitype.StackTagName]string,
	teams []string,
	state *apitype.UntypedDeployment,
	config *apitype.StackConfig,
) (apitype.Stack, CreateStackDetails, error) {
	// Validate names and tags.
	if err := validation.ValidateStackTags(tags); err != nil {
		return apitype.Stack{}, CreateStackDetails{}, fmt.Errorf("validating stack properties: %w", err)
	}

	stack := apitype.Stack{
		StackName:   stackID.Stack.Q(),
		ProjectName: stackID.Project,
		OrgName:     stackID.Owner,
		Tags:        tags,
	}
	createStackReq := apitype.CreateStackRequest{
		StackName: stackID.Stack.String(),
		Tags:      tags,
		Teams:     teams,
		State:     state,
		Config:    config,
	}

	var createStackResp apitype.CreateStackResponse
	endpoint := fmt.Sprintf("/api/stacks/%s/%s", stackID.Owner, stackID.Project)
	if err := pc.restCall(
		ctx, "POST", endpoint, nil, &createStackReq, &createStackResp); err != nil {
		return apitype.Stack{}, CreateStackDetails{}, err
	}

	return stack, CreateStackDetails{
		Messages: createStackResp.Messages,
	}, nil
}

// DeleteStack deletes the indicated stack. If force is true, the stack is deleted even if it contains resources.
func (pc *Client) DeleteStack(ctx context.Context, stack StackIdentifier, force bool) (bool, error) {
	path := getStackPath(stack)
	queryObj := struct {
		Force bool `url:"force"`
	}{
		Force: force,
	}

	err := pc.restCall(ctx, "DELETE", path, queryObj, nil, nil)
	return isStackHasResourcesError(err), err
}

func isStackHasResourcesError(err error) bool {
	if err == nil {
		return false
	}

	errRsp, ok := err.(*apitype.ErrorResponse)
	if !ok {
		return false
	}

	return errRsp.Code == 400 && errRsp.Message == "Bad Request: Stack still contains resources."
}

// EncryptValue encrypts a plaintext value in the context of the indicated stack.
func (pc *Client) EncryptValue(ctx context.Context, stack StackIdentifier, plaintext []byte) ([]byte, error) {
	req := apitype.EncryptValueRequest{Plaintext: plaintext}
	var resp apitype.EncryptValueResponse
	if err := pc.restCallWithOptions(
		ctx, "POST", getStackPath(stack, "encrypt"), nil, &req, &resp,
		httpCallOptions{RetryPolicy: retryAllMethods},
	); err != nil {
		return nil, err
	}
	return resp.Ciphertext, nil
}

// BatchEncrypt encrypts multiple plaintext values in the context of the indicated stack.
func (pc *Client) BatchEncrypt(ctx context.Context, stack StackIdentifier,
	plaintexts [][]byte,
) ([][]byte, error) {
	req := apitype.BatchEncryptRequest{Plaintexts: plaintexts}
	var resp apitype.BatchEncryptResponse
	if err := pc.restCallWithOptions(ctx, "POST", getStackPath(stack, "batch-encrypt"), nil, &req, &resp,
		httpCallOptions{GzipCompress: true, RetryPolicy: retryAllMethods}); err != nil {
		return nil, err
	}

	return resp.Ciphertexts, nil
}

// DecryptValue decrypts a ciphertext value in the context of the indicated stack.
func (pc *Client) DecryptValue(ctx context.Context, stack StackIdentifier, ciphertext []byte) ([]byte, error) {
	req := apitype.DecryptValueRequest{Ciphertext: ciphertext}
	var resp apitype.DecryptValueResponse
	if err := pc.restCallWithOptions(
		ctx, "POST", getStackPath(stack, "decrypt"), nil, &req, &resp,
		httpCallOptions{RetryPolicy: retryAllMethods},
	); err != nil {
		return nil, err
	}
	return resp.Plaintext, nil
}

func (pc *Client) Log3rdPartySecretsProviderDecryptionEvent(ctx context.Context, stack StackIdentifier,
	secretName string,
) error {
	req := apitype.Log3rdPartyDecryptionEvent{SecretName: secretName}
	return pc.restCall(
		ctx, "POST", path.Join(getStackPath(stack, "decrypt"), "log-decryption"),
		nil, &req, nil)
}

func (pc *Client) LogBulk3rdPartySecretsProviderDecryptionEvent(ctx context.Context, stack StackIdentifier,
	command string,
) error {
	req := apitype.Log3rdPartyDecryptionEvent{CommandName: command}
	return pc.restCall(
		ctx, "POST", path.Join(getStackPath(stack, "decrypt"), "log-batch-decryption"),
		nil, &req, nil)
}

// BatchDecryptValue decrypts a ciphertext value in the context of the indicated stack.
func (pc *Client) BatchDecryptValue(ctx context.Context, stack StackIdentifier,
	ciphertexts [][]byte,
) (map[string][]byte, error) {
	req := apitype.BatchDecryptRequest{Ciphertexts: ciphertexts}
	var resp apitype.BatchDecryptResponse
	if err := pc.restCallWithOptions(ctx, "POST", getStackPath(stack, "batch-decrypt"), nil, &req, &resp,
		httpCallOptions{GzipCompress: true, RetryPolicy: retryAllMethods}); err != nil {
		return nil, err
	}

	return resp.Plaintexts, nil
}

// GetStackUpdates returns all updates to the indicated stack.
func (pc *Client) GetStackUpdates(
	ctx context.Context,
	stack StackIdentifier,
	pageSize int,
	page int,
) ([]apitype.UpdateInfo, error) {
	var response apitype.GetHistoryResponse
	path := getStackPath(stack, "updates")
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		path += fmt.Sprintf("?pageSize=%d&page=%d", pageSize, page)
	}
	if err := pc.restCall(ctx, "GET", path, nil, nil, &response); err != nil {
		return nil, err
	}

	return response.Updates, nil
}

// GetLatestStackPreviews returns the stack's most recent preview operations, newest-first.
// Previews are tracked separately from update history (see GetStackUpdates).
func (pc *Client) GetLatestStackPreviews(
	ctx context.Context,
	stack StackIdentifier,
) ([]apitype.StackPreview, error) {
	var response apitype.GetLatestStackPreviewsResponse
	// asc=false requests newest-first; the endpoint otherwise defaults to oldest-first.
	path := getStackPath(stack, "updates", "latest", "previews") + "?asc=false&pageSize=1&page=1"
	if err := pc.restCall(ctx, "GET", path, nil, nil, &response); err != nil {
		return nil, err
	}

	return response.Updates, nil
}

// ExportStackDeployment exports the indicated stack's deployment as a raw JSON message.
// If version is nil, will export the latest version of the stack.
func (pc *Client) ExportStackDeployment(
	ctx context.Context, stack StackIdentifier, version *int,
) (apitype.UntypedDeployment, error) {
	tracingSpan, childCtx := opentracing.StartSpanFromContext(ctx, "ExportStackDeployment")
	defer tracingSpan.Finish()

	tracer := otel.Tracer("pulumi-cli")
	childCtx, otelSpan := cmdutil.StartSpan(childCtx, tracer, "ExportStackDeployment")
	defer otelSpan.End()

	path := getStackPath(stack, "export")

	// Tack on a specific version as desired.
	if version != nil {
		path += fmt.Sprintf("/%d", *version)
	}

	var resp apitype.ExportStackResponse
	if err := pc.restCall(childCtx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.UntypedDeployment{}, err
	}

	return apitype.UntypedDeployment(resp), nil
}

// ImportStackDeployment imports a new deployment into the indicated stack.
func (pc *Client) ImportStackDeployment(ctx context.Context, stack StackIdentifier,
	deployment *apitype.UntypedDeployment,
) (UpdateIdentifier, error) {
	var resp apitype.ImportStackResponse
	if err := pc.restCallWithOptions(ctx, "POST", getStackPath(stack, "import"), nil, deployment, &resp,
		httpCallOptions{GzipCompress: true}); err != nil {
		return UpdateIdentifier{}, err
	}

	return UpdateIdentifier{
		StackIdentifier: stack,
		UpdateKind:      apitype.UpdateUpdate,
		UpdateID:        resp.UpdateID,
	}, nil
}

type CreateUpdateDetails struct {
	Messages                []apitype.Message
	RequiredPolicies        []apitype.RequiredPolicy
	IsNeoIntegrationEnabled bool
}

// CreateUpdate creates a new update for the indicated stack with the given kind and assorted options. If the update
// requires that the Pulumi program is uploaded, the provided getContents callback will be invoked to fetch the
// contents of the Pulumi program.
func (pc *Client) CreateUpdate(
	ctx context.Context, kind apitype.UpdateKind, stack StackIdentifier, proj *workspace.Project,
	cfg config.Map, m apitype.UpdateMetadata, opts engine.UpdateOptions,
	dryRun bool,
) (UpdateIdentifier, CreateUpdateDetails, error) {
	// First create the update program request.
	wireConfig := make(map[string]apitype.ConfigValue)
	for k, cv := range cfg {
		v, err := cv.Value(config.NopDecrypter)
		contract.AssertNoErrorf(err, "error fetching config value for key %v", k)

		wireConfig[k.String()] = apitype.ConfigValue{
			String: v,
			Secret: cv.Secure(),
			Object: cv.Object(),
		}
	}

	description := ""
	if proj.Description != nil {
		description = *proj.Description
	}

	updateRequest := apitype.UpdateProgramRequest{
		Name:        string(proj.Name),
		Runtime:     proj.Runtime.Name(),
		Main:        proj.Main,
		Description: description,
		Config:      wireConfig,
		Options: apitype.UpdateOptions{
			LocalPolicyPackPaths: engine.ConvertLocalPolicyPacksToPaths(opts.LocalPolicyPacks),
			Color:                colors.Raw, // force raw colorization, we handle colorization in the CLI
			DryRun:               dryRun,
			Parallel:             opts.Parallel,
			ShowConfig:           false, // This is a legacy option now, the engine will always emit config information
			ShowReplacementSteps: false, // This is a legacy option now, the engine will always emit this information
			ShowSames:            false, // This is a legacy option now, the engine will always emit this information
		},
		Metadata: m,
	}

	// Create the initial update object.
	var endpoint string
	switch kind {
	case apitype.UpdateUpdate, apitype.ResourceImportUpdate:
		endpoint = "update"
	case apitype.PreviewUpdate:
		endpoint = "preview"
	case apitype.RefreshUpdate:
		endpoint = "refresh"
	case apitype.DestroyUpdate:
		endpoint = "destroy"
	case apitype.StackImportUpdate, apitype.RenameUpdate:
		contract.Failf("%s updates are not supported", kind)
	default:
		contract.Failf("Unknown kind: %s", kind)
	}

	path := getStackPath(stack, endpoint)
	var updateResponse apitype.UpdateProgramResponse
	if err := pc.restCall(ctx, "POST", path, nil, &updateRequest, &updateResponse); err != nil {
		return UpdateIdentifier{}, CreateUpdateDetails{}, err
	}

	return UpdateIdentifier{
			StackIdentifier: stack,
			UpdateKind:      kind,
			UpdateID:        updateResponse.UpdateID,
		}, CreateUpdateDetails{
			Messages:                updateResponse.Messages,
			RequiredPolicies:        updateResponse.RequiredPolicies,
			IsNeoIntegrationEnabled: updateResponse.AISettings.CopilotIsEnabled,
		}, nil
}

// RenameStack renames the provided stack to have the new identifier.
func (pc *Client) RenameStack(ctx context.Context, currentID, newID StackIdentifier) error {
	req := apitype.StackRenameRequest{
		NewName:    newID.Stack.String(),
		NewProject: newID.Project,
	}
	return pc.restCall(ctx, "POST", getStackPath(currentID, "rename"), nil, &req, nil)
}

// StartUpdate starts the indicated update. It returns the new version of the update's target stack and the token used
// to authenticate operations on the update if any. Replaces the stack's tags with the updated set.
func (pc *Client) StartUpdate(ctx context.Context, update UpdateIdentifier,
	tags map[apitype.StackTagName]string,
) (int, string, int64, error) {
	// Validate names and tags.
	if err := validation.ValidateStackTags(tags); err != nil {
		return 0, "", 0, fmt.Errorf("validating stack properties: %w", err)
	}

	req := apitype.StartUpdateRequest{
		Tags: tags,
	}

	if !env.DisableJournaling.Value() {
		req.JournalVersion = apitype.LatestJournalVersion
	}

	var resp apitype.StartUpdateResponse
	if err := pc.restCall(ctx, "POST", getUpdatePath(update), nil, req, &resp); err != nil {
		return 0, "", 0, err
	}

	return resp.Version, resp.Token, resp.JournalVersion, nil
}

// ListPolicyGroups lists all `PolicyGroups` the organization has in the Pulumi service.
func (pc *Client) ListPolicyGroups(ctx context.Context, orgName string, inContToken *string) (
	apitype.ListPolicyGroupsResponse, *string, error,
) {
	// NOTE: The ListPolicyGroups API on the Pulumi Service is not currently paginated.
	var resp apitype.ListPolicyGroupsResponse
	err := pc.restCall(ctx, "GET", listPolicyGroupsPath(orgName), nil, nil, &resp)
	if err != nil {
		return resp, nil, fmt.Errorf("List Policy Groups failed: %w", err)
	}
	return resp, nil, nil
}

// CreatePolicyGroup creates a new Policy Group in the given organization.
func (pc *Client) CreatePolicyGroup(
	ctx context.Context, orgName string, req apitype.CreatePolicyGroupRequest,
) error {
	if err := pc.restCall(ctx, "POST", listPolicyGroupsPath(orgName), nil, req, nil); err != nil {
		return fmt.Errorf("creating policy group: %w", err)
	}
	return nil
}

// GetPolicyGroup returns the details of a single Policy Group in the Pulumi
// service, including the list of Policy Packs applied to it and the stacks or
// cloud accounts that are members of the group.
func (pc *Client) GetPolicyGroup(
	ctx context.Context, orgName, policyGroup string,
) (apitype.GetPolicyGroupResponse, error) {
	var resp apitype.GetPolicyGroupResponse
	err := pc.restCall(ctx, "GET", updatePolicyGroupPath(orgName, policyGroup), nil, nil, &resp)
	if err != nil {
		return resp, fmt.Errorf("getting policy group: %w", err)
	}
	return resp, nil
}

// UpdatePolicyGroup issues a PATCH against the Policy Group endpoint. The
// service's UpdatePolicyGroup endpoint accepts at most one mutation per
// request (rename, add/remove stack, add/remove policy pack, add/remove
// insights account), so callers performing multiple mutations must issue
// multiple calls.
func (pc *Client) UpdatePolicyGroup(
	ctx context.Context, orgName, policyGroup string, req apitype.UpdatePolicyGroupRequest,
) error {
	if err := pc.restCall(
		ctx, http.MethodPatch, updatePolicyGroupPath(orgName, policyGroup), nil, req, nil,
	); err != nil {
		return fmt.Errorf("updating policy group: %w", err)
	}
	return nil
}

// DeletePolicyGroup deletes a Policy Group from the given organization. The
// organization's default Policy Group cannot be deleted; the service will
// reject such requests.
func (pc *Client) DeletePolicyGroup(ctx context.Context, orgName, policyGroup string) error {
	if err := pc.restCall(
		ctx, http.MethodDelete, updatePolicyGroupPath(orgName, policyGroup), nil, nil, nil,
	); err != nil {
		return fmt.Errorf("removing policy group: %w", err)
	}
	return nil
}

// ListOrganizationMembers returns a single page of members for the given
// organization, wrapping the `ListOrganizationMembers` Pulumi Cloud REST
// endpoint (GET /api/orgs/{orgName}/members).
//
// mode selects between "frontend" members (data stored in the Pulumi Service's
// database) and "backend" members (data stored in the organization's identity
// backend, e.g. GitHub or GitLab). When mode is empty, the service default is
// used. continuationToken pages through results when non-nil; pass the
// ContinuationToken returned by a previous response to fetch the next page.
func (pc *Client) ListOrganizationMembers(
	ctx context.Context, orgName, mode string, continuationToken *string,
) (apitype.ListOrganizationMembersResponse, error) {
	queryObj := struct {
		Type              string  `url:"type,omitempty"`
		ContinuationToken *string `url:"continuationToken,omitempty"`
	}{
		Type:              mode,
		ContinuationToken: continuationToken,
	}

	var resp apitype.ListOrganizationMembersResponse
	path := fmt.Sprintf("/api/orgs/%s/members", url.PathEscape(orgName))
	if err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &resp); err != nil {
		return resp, fmt.Errorf("listing organization members: %w", err)
	}
	return resp, nil
}

// ListAuditLogsOptions are the optional query parameters accepted by
// ListAuditLogs. Empty fields are omitted from the request and let the
// service apply its own defaults.
type ListAuditLogsOptions struct {
	// EventType filters the audit log to a single event type (e.g.
	// "stack.create"). Empty means no filter.
	EventType string
	// User filters the audit log to events triggered by a single user, by
	// GitHub login. Empty means no filter.
	User string
	// StartTime is the upper-bound timestamp of the time range to query, as
	// understood by the V1 endpoint. Empty means the service default.
	StartTime string
	// ContinuationToken pages through results; pass the ContinuationToken
	// returned by a previous response to fetch the next page.
	ContinuationToken string
}

// ListAuditLogs returns a single page of audit log events for the given
// organization, wrapping the `ListAuditLogEvents` Pulumi Cloud REST endpoint
// (GET /api/orgs/{orgName}/auditlogs).
func (pc *Client) ListAuditLogs(
	ctx context.Context, orgName string, opts ListAuditLogsOptions,
) (apitype.ListAuditLogEventsResponse, error) {
	queryObj := struct {
		EventType         string `url:"eventType,omitempty"`
		User              string `url:"user,omitempty"`
		StartTime         string `url:"startTime,omitempty"`
		ContinuationToken string `url:"continuationToken,omitempty"`
	}{
		EventType:         opts.EventType,
		User:              opts.User,
		StartTime:         opts.StartTime,
		ContinuationToken: opts.ContinuationToken,
	}

	var resp apitype.ListAuditLogEventsResponse
	path := fmt.Sprintf("/api/orgs/%s/auditlogs", url.PathEscape(orgName))
	if err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &resp); err != nil {
		return resp, fmt.Errorf("listing audit logs: %w", err)
	}
	return resp, nil
}

// ExportAuditLogsOptions are the optional query parameters accepted by
// ExportAuditLogs. Empty fields are omitted from the request and let the
// service apply its own defaults.
type ExportAuditLogsOptions struct {
	// Format is the export format: "csv" or "cef". Empty defaults to "csv".
	Format string
	// EventType filters the audit log to a single event type. Empty means no
	// filter.
	EventType string
	// User filters the audit log to events triggered by a single user, by
	// GitHub login. Empty means no filter.
	User string
	// StartTime is the upper-bound timestamp of the time range to query, as
	// understood by the V1 endpoint. Empty means the service default.
	StartTime string
	// ContinuationToken pages through results; pass the ContinuationToken
	// returned by a previous response to fetch the next page.
	ContinuationToken string
}

// ExportAuditLogs streams an export of audit log events for the given
// organization in the requested format (csv or cef), wrapping the
// `ExportAuditLogEvents` Pulumi Cloud REST endpoint
// (GET /api/orgs/{orgName}/auditlogs/export). Unlike ListAuditLogs, the
// response is plain text (CSV or CEF lines), not JSON; the caller is
// responsible for closing the returned ReadCloser.
func (pc *Client) ExportAuditLogs(
	ctx context.Context, orgName string, opts ExportAuditLogsOptions,
) (io.ReadCloser, error) {
	format := opts.Format
	if format == "" {
		format = "csv"
	}
	queryObj := struct {
		Format            string `url:"format,omitempty"`
		EventType         string `url:"eventType,omitempty"`
		User              string `url:"user,omitempty"`
		StartTime         string `url:"startTime,omitempty"`
		ContinuationToken string `url:"continuationToken,omitempty"`
	}{
		Format:            format,
		EventType:         opts.EventType,
		User:              opts.User,
		StartTime:         opts.StartTime,
		ContinuationToken: opts.ContinuationToken,
	}

	var body io.ReadCloser
	path := fmt.Sprintf("/api/orgs/%s/auditlogs/export", url.PathEscape(orgName))
	if err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &body); err != nil {
		return nil, fmt.Errorf("exporting audit logs: %w", err)
	}
	return body, nil
}

// UpdateOrganizationMember updates the role assignment of a member within
// the given organization. Wraps the `UpdateOrganizationMember` Pulumi Cloud
// REST endpoint (PATCH /api/orgs/{orgName}/members/{userLogin}). Only the
// non-nil fields of req are sent; the service interprets omitted fields as
// "leave unchanged".
func (pc *Client) UpdateOrganizationMember(
	ctx context.Context, orgName, userLogin string, req apitype.UpdateOrganizationMemberRequest,
) error {
	path := fmt.Sprintf("/api/orgs/%s/members/%s", url.PathEscape(orgName), url.PathEscape(userLogin))
	if err := pc.restCall(ctx, http.MethodPatch, path, nil, req, nil); err != nil {
		return fmt.Errorf("updating organization member: %w", err)
	}
	return nil
}

// RemoveOrganizationMember removes a user from the given organization. The
// removed user loses access to all organization resources including stacks,
// teams, and projects. Wraps the `DeleteOrganizationMember` Pulumi Cloud REST
// endpoint (DELETE /api/orgs/{orgName}/members/{userLogin}).
func (pc *Client) RemoveOrganizationMember(ctx context.Context, orgName, userLogin string) error {
	path := fmt.Sprintf("/api/orgs/%s/members/%s", url.PathEscape(orgName), url.PathEscape(userLogin))
	if err := pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil); err != nil {
		return fmt.Errorf("removing organization member: %w", err)
	}
	return nil
}

// ListPolicyIssuesOptions are the optional pagination parameters accepted by
// ListPolicyIssues. Zero values mean "let the server pick the default":
// Page < 1 → 1, PageSize ≤ 0 → server default, Asc false → descending order.
// ListPolicyIssuesOptions configures the policy issues list request.
type ListPolicyIssuesOptions struct {
	// StartRow is the 0-based offset of the first result.
	StartRow int
	// EndRow is the exclusive upper bound (startRow + pageSize).
	EndRow int
}

// ListPolicyIssues returns a paginated list of policy issues for the given
// organization, wrapping the `ListPolicyIssues` Pulumi Cloud REST endpoint
// (POST /api/orgs/{orgName}/policyresults/issues). The endpoint uses POST
// because the request body carries pagination parameters in an AngularGrid
// format.
func (pc *Client) ListPolicyIssues(
	ctx context.Context, orgName string, opts ListPolicyIssuesOptions,
) (apitype.ListPolicyIssuesResponse, error) {
	req := apitype.ListPolicyIssuesRequest{
		StartRow: opts.StartRow,
		EndRow:   opts.EndRow,
	}
	path := fmt.Sprintf("/api/orgs/%s/policyresults/issues", url.PathEscape(orgName))

	var resp apitype.ListPolicyIssuesResponse
	if err := pc.restCall(ctx, http.MethodPost, path, nil, req, &resp); err != nil {
		return apitype.ListPolicyIssuesResponse{}, fmt.Errorf("listing policy issues: %w", err)
	}
	return resp, nil
}

// GetPolicyComplianceResults returns compliance results for policy issues
// grouped by entity.
func (pc *Client) GetPolicyComplianceResults(
	ctx context.Context, orgName string, req apitype.GetPolicyComplianceResultsRequest,
) (apitype.GetPolicyComplianceResultsResponse, error) {
	path := fmt.Sprintf("/api/orgs/%s/policyresults/compliance", url.PathEscape(orgName))
	var resp apitype.GetPolicyComplianceResultsResponse
	if err := pc.restCall(ctx, http.MethodPost, path, nil, req, &resp); err != nil {
		return apitype.GetPolicyComplianceResultsResponse{},
			fmt.Errorf("getting policy compliance results: %w", err)
	}
	return resp, nil
}

// GetPolicyIssue returns the details of a single policy issue in the given
// organization, wrapping the `GetPolicyIssue` Pulumi Cloud REST endpoint
// (GET /api/orgs/{orgName}/policyresults/issues/{issueId}).
func (pc *Client) GetPolicyIssue(
	ctx context.Context, orgName, issueID string,
) (apitype.PolicyIssue, error) {
	path := fmt.Sprintf(
		"/api/orgs/%s/policyresults/issues/%s",
		url.PathEscape(orgName), url.PathEscape(issueID))

	var resp apitype.GetPolicyIssueResponse
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return apitype.PolicyIssue{}, fmt.Errorf("getting policy issue: %w", err)
	}
	return resp.PolicyIssue, nil
}

// ListPolicyPacks lists all `PolicyPack` the organization has in the Pulumi service.
func (pc *Client) ListPolicyPacks(ctx context.Context, orgName string, inContToken *string) (
	apitype.ListPolicyPacksResponse, *string, error,
) {
	// NOTE: The ListPolicyPacks API on the Pulumi Service is not currently paginated.
	var resp apitype.ListPolicyPacksResponse
	err := pc.restCall(ctx, "GET", listPolicyPacksPath(orgName), nil, nil, &resp)
	if err != nil {
		return resp, nil, fmt.Errorf("List Policy Packs failed: %w", err)
	}
	return resp, nil, nil
}

// ListOrgRoles lists the custom roles defined in the given organization, optionally
// filtered by their UX purpose (e.g. "organization", "team", "token"). An empty
// uxPurpose returns all roles. The Pulumi Cloud REST API is not paginated for
// this endpoint, so all roles are returned in a single call.
func (pc *Client) ListOrgRoles(
	ctx context.Context, orgName, uxPurpose string,
) ([]apitype.Role, error) {
	path := fmt.Sprintf("/api/orgs/%s/roles", url.PathEscape(orgName))
	queryObj := struct {
		UXPurpose string `url:"uxPurpose,omitempty"`
	}{UXPurpose: uxPurpose}

	var resp apitype.ListRolesResponse
	if err := pc.restCall(ctx, "GET", path, queryObj, nil, &resp); err != nil {
		return nil, fmt.Errorf("listing organization roles: %w", err)
	}
	return resp.Roles, nil
}

// CreateOrgRole creates a new custom role in the given organization.
func (pc *Client) CreateOrgRole(
	ctx context.Context, orgName string, req apitype.CreateRoleRequest,
) (apitype.Role, error) {
	path := fmt.Sprintf("/api/orgs/%s/roles", url.PathEscape(orgName))
	var resp apitype.Role
	if err := pc.restCall(ctx, "POST", path, nil, &req, &resp); err != nil {
		return apitype.Role{}, fmt.Errorf("creating organization role: %w", err)
	}
	return resp, nil
}

// GetOrgRole fetches a single custom role by its identifier.
func (pc *Client) GetOrgRole(
	ctx context.Context, orgName, roleID string,
) (apitype.Role, error) {
	path := fmt.Sprintf("/api/orgs/%s/roles/%s",
		url.PathEscape(orgName), url.PathEscape(roleID))
	var resp apitype.Role
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.Role{}, fmt.Errorf("getting organization role: %w", err)
	}
	return resp, nil
}

// UpdateOrgRole updates an existing custom role's name, description, and details.
// The service requires all three fields; callers that want to leave any of them
// unchanged should fetch the current role first and merge.
func (pc *Client) UpdateOrgRole(
	ctx context.Context, orgName, roleID string, req apitype.UpdateRoleRequest,
) (apitype.Role, error) {
	path := fmt.Sprintf("/api/orgs/%s/roles/%s",
		url.PathEscape(orgName), url.PathEscape(roleID))
	var resp apitype.Role
	if err := pc.restCall(ctx, "PATCH", path, nil, &req, &resp); err != nil {
		return apitype.Role{}, fmt.Errorf("updating organization role: %w", err)
	}
	return resp, nil
}

// DeleteOrgRole deletes a custom role from an organization. When force is true,
// the service will delete the role even if it is currently assigned to members
// or teams (and revoke those assignments).
func (pc *Client) DeleteOrgRole(
	ctx context.Context, orgName, roleID string, force bool,
) error {
	path := fmt.Sprintf("/api/orgs/%s/roles/%s",
		url.PathEscape(orgName), url.PathEscape(roleID))
	queryObj := struct {
		Force bool `url:"force,omitempty"`
	}{Force: force}
	if err := pc.restCall(ctx, "DELETE", path, queryObj, nil, nil); err != nil {
		return fmt.Errorf("deleting organization role: %w", err)
	}
	return nil
}

// AssignTeamRole upserts the role assignment for the given team. The Pulumi
// Cloud REST API currently supports a single role per team, so calling this
// method replaces any previously assigned custom role.
func (pc *Client) AssignTeamRole(
	ctx context.Context, orgName, teamName, roleID string,
) error {
	path := fmt.Sprintf("/api/orgs/%s/teams/%s/roles/%s",
		url.PathEscape(orgName), url.PathEscape(teamName), url.PathEscape(roleID))
	if err := pc.restCall(ctx, "POST", path, nil, nil, nil); err != nil {
		return fmt.Errorf("assigning role to team: %w", err)
	}
	return nil
}

// PublishPolicyPack publishes a `PolicyPack` to the Pulumi service. If it successfully publishes
// the Policy Pack, it returns the version of the pack.
func (pc *Client) PublishPolicyPack(ctx context.Context, orgName string,
	runtime string, analyzerInfo plugin.AnalyzerInfo, dirArchive io.Reader,
	metadata map[string]string,
) (string, error) {
	//
	// Step 1: Send POST containing policy metadata to service. This begins process of creating
	// publishing the PolicyPack.
	//

	if err := validatePolicyPackVersion(analyzerInfo.Version); err != nil {
		return "", err
	}

	policies := make([]apitype.Policy, len(analyzerInfo.Policies))
	for i, policy := range analyzerInfo.Policies {
		configSchema, err := convertPolicyConfigSchema(policy.ConfigSchema)
		if err != nil {
			return "", err
		}

		policies[i] = apitype.Policy{
			Name:             policy.Name,
			DisplayName:      policy.DisplayName,
			Description:      policy.Description,
			EnforcementLevel: policy.EnforcementLevel,
			Message:          policy.Message,
			ConfigSchema:     configSchema,
			Severity:         policy.Severity,
			Framework:        convertPolicyComplianceFramework(policy.Framework),
			Tags:             policy.Tags,
			RemediationSteps: policy.RemediationSteps,
			URL:              policy.URL,
		}
	}

	req := apitype.CreatePolicyPackRequest{
		Name:        analyzerInfo.Name,
		DisplayName: analyzerInfo.DisplayName,
		VersionTag:  analyzerInfo.Version,
		Policies:    policies,
		Description: analyzerInfo.Description,
		Readme:      analyzerInfo.Readme,
		Provider:    analyzerInfo.Provider,
		Tags:        analyzerInfo.Tags,
		Repository:  analyzerInfo.Repository,
		Runtime:     runtime,
		Metadata:    metadata,
	}

	// Print a publishing message. We have to handle the case where an older version of pulumi/policy
	// is in use, which does not provide  a version tag.
	var versionMsg string
	if analyzerInfo.Version != "" {
		versionMsg = " - version " + analyzerInfo.Version
	}
	fmt.Printf("Publishing %q%s to %q\n", analyzerInfo.Name, versionMsg, orgName)

	var resp apitype.CreatePolicyPackResponse
	err := pc.restCall(ctx, "POST", publishPolicyPackPath(orgName), nil, req, &resp)
	if err != nil {
		return "", fmt.Errorf("Publish policy pack failed: %w", err)
	}

	//
	// Step 2: Upload the compressed PolicyPack directory to the pre-signed object storage service URL.
	// The PolicyPack is now published.
	//

	putReq, err := http.NewRequest(http.MethodPut, resp.UploadURI, dirArchive)
	if err != nil {
		return "", fmt.Errorf("Failed to upload compressed PolicyPack: %w", err)
	}

	for k, v := range resp.RequiredHeaders {
		putReq.Header.Add(k, v)
	}

	_, err = pc.restClient.HTTPClient().Do(putReq, retryAllMethods)
	if err != nil {
		return "", fmt.Errorf("Failed to upload compressed PolicyPack: %w", err)
	}

	//
	// Step 3: Signal to the service that the PolicyPack publish operation is complete.
	//

	// If the version tag is empty, an older version of pulumi/policy is being used and
	// we therefore need to use the version provided by the pulumi service.
	version := analyzerInfo.Version
	if version == "" {
		version = strconv.Itoa(resp.Version)
		fmt.Printf("Published as version %s\n", version)
	}
	err = pc.restCall(ctx, "POST",
		publishPolicyPackPublishComplete(orgName, analyzerInfo.Name, version), nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("Request to signal completion of the publish operation failed: %w", err)
	}

	return version, nil
}

// convertPolicyConfigSchema converts a policy's schema from the analyzer to the apitype.
func convertPolicyConfigSchema(schema *plugin.AnalyzerPolicyConfigSchema) (*apitype.PolicyConfigSchema, error) {
	if schema == nil {
		return nil, nil
	}
	properties := map[string]*json.RawMessage{}
	for k, v := range schema.Properties {
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		raw := json.RawMessage(bytes)
		properties[k] = &raw
	}
	return &apitype.PolicyConfigSchema{
		Type:       apitype.Object,
		Properties: properties,
		Required:   schema.Required,
	}, nil
}

// convertPolicyComplianceFramework converts a policy compliance framework from the analyzer to the apitype.
func convertPolicyComplianceFramework(f *plugin.AnalyzerPolicyComplianceFramework) *apitype.PolicyComplianceFramework {
	if f == nil {
		return nil
	}
	return &apitype.PolicyComplianceFramework{
		Name:          f.Name,
		Version:       f.Version,
		Reference:     f.Reference,
		Specification: f.Specification,
	}
}

// validatePolicyPackVersion validates the version of a Policy Pack. The version may be empty,
// as it is likely an older version of pulumi/policy that does not gather the version.
func validatePolicyPackVersion(s string) error {
	if s == "" {
		return nil
	}

	policyPackVersionTagRE := regexp.MustCompile("^[a-zA-Z0-9-_.]{1,100}$")
	if !policyPackVersionTagRE.MatchString(s) {
		msg := fmt.Sprintf("invalid version %q - version may only contain alphanumeric, hyphens, or underscores. "+
			"It must also be between 1 and 100 characters long.", s)
		return errors.New(msg)
	}
	return nil
}

// ApplyPolicyPack enables a `PolicyPack` to the Pulumi organization. If policyGroup is not empty,
// it will enable the PolicyPack on the default PolicyGroup.
func (pc *Client) ApplyPolicyPack(ctx context.Context, orgName, policyGroup,
	policyPackName, versionTag string, policyPackConfig map[string]*json.RawMessage,
) error {
	// If a Policy Group was not specified, we use the default Policy Group.
	if policyGroup == "" {
		policyGroup = apitype.DefaultPolicyGroup
	}

	req := apitype.UpdatePolicyGroupRequest{
		AddPolicyPack: &apitype.PolicyPackMetadata{
			Name:       policyPackName,
			VersionTag: versionTag,
			Config:     policyPackConfig,
		},
	}

	err := pc.restCall(ctx, http.MethodPatch, updatePolicyGroupPath(orgName, policyGroup), nil, req, nil)
	if err != nil {
		return fmt.Errorf("Enable policy pack failed: %w", err)
	}
	return nil
}

// GetPolicyPackSchema gets Policy Pack config schema.
func (pc *Client) GetPolicyPackSchema(ctx context.Context, orgName,
	policyPackName, versionTag string,
) (*apitype.GetPolicyPackConfigSchemaResponse, error) {
	var resp apitype.GetPolicyPackConfigSchemaResponse
	err := pc.restCall(ctx, http.MethodGet,
		getPolicyPackConfigSchemaPath(orgName, policyPackName, versionTag), nil, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("Retrieving policy pack config schema failed: %w", err)
	}
	return &resp, nil
}

// DisablePolicyPack disables a `PolicyPack` to the Pulumi organization. If policyGroup is not empty,
// it will disable the PolicyPack on the default PolicyGroup.
func (pc *Client) DisablePolicyPack(ctx context.Context, orgName string, policyGroup string,
	policyPackName, versionTag string,
) error {
	// If Policy Group was not specified, use the default Policy Group.
	if policyGroup == "" {
		policyGroup = apitype.DefaultPolicyGroup
	}

	req := apitype.UpdatePolicyGroupRequest{
		RemovePolicyPack: &apitype.PolicyPackMetadata{
			Name:       policyPackName,
			VersionTag: versionTag,
		},
	}

	err := pc.restCall(ctx, http.MethodPatch, updatePolicyGroupPath(orgName, policyGroup), nil, req, nil)
	if err != nil {
		return fmt.Errorf("Request to disable policy pack failed: %w", err)
	}
	return nil
}

// RemovePolicyPack removes all versions of a `PolicyPack` from the Pulumi organization.
func (pc *Client) RemovePolicyPack(ctx context.Context, orgName string, policyPackName string) error {
	path := deletePolicyPackPath(orgName, policyPackName)
	err := pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("Request to remove policy pack failed: %w", err)
	}
	return nil
}

// RemovePolicyPackByVersion removes a specific version of a `PolicyPack` from
// the Pulumi organization.
func (pc *Client) RemovePolicyPackByVersion(ctx context.Context, orgName string,
	policyPackName string, versionTag string,
) error {
	path := deletePolicyPackVersionPath(orgName, policyPackName, versionTag)
	err := pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("Request to remove policy pack failed: %w", err)
	}
	return nil
}

// GetStackPolicyPacks gets the required policy packs currently applicable to the stack.
func (pc *Client) GetStackPolicyPacks(ctx context.Context,
	stackID StackIdentifier,
) (apitype.GetStackPolicyPacksResponse, error) {
	var resp apitype.GetStackPolicyPacksResponse
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "policypacks"), nil, nil, &resp); err != nil {
		return apitype.GetStackPolicyPacksResponse{}, err
	}
	return resp, nil
}

// DownloadPolicyPack downloads a `PolicyPack` from the given URL. It returns a ReadCloser to read
// the PolicyPack and the content length. A content length of -1 indicates that the length is unknown.
func (pc *Client) DownloadPolicyPack(ctx context.Context, url string) (io.ReadCloser, int64, error) {
	getS3Req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to download compressed PolicyPack: %w", err)
	}

	resp, err := pc.restClient.HTTPClient().Do(getS3Req, retryAllMethods)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to download compressed PolicyPack: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, -1, fmt.Errorf("Failed to download compressed PolicyPack: %s", resp.Status)
	}

	return resp.Body, resp.ContentLength, nil
}

// GetUpdateEvents returns all events, taking an optional continuation token from a previous call.
func (pc *Client) GetUpdateEvents(ctx context.Context, update UpdateIdentifier,
	continuationToken *string,
) (apitype.UpdateResults, error) {
	path := getUpdatePath(update)
	if continuationToken != nil {
		path += "?continuationToken=" + *continuationToken
	}

	var results apitype.UpdateResults
	if err := pc.restCall(ctx, "GET", path, nil, nil, &results); err != nil {
		return apitype.UpdateResults{}, err
	}

	return results, nil
}

// RenewUpdateLease renews the indicated update lease for the given duration.
func (pc *Client) RenewUpdateLease(ctx context.Context, update UpdateIdentifier, token string,
	duration time.Duration,
) (string, error) {
	req := apitype.RenewUpdateLeaseRequest{
		Duration: int(duration / time.Second),
	}
	var resp apitype.RenewUpdateLeaseResponse

	// While renewing a lease uses POST, it is safe to send multiple requests (consider that we do this multiple times
	// during a long running update).  Since we would fail our update operation if we can't renew our lease, we'll retry
	// these POST operations.
	if err := pc.updateRESTCall(ctx, "POST", getUpdatePath(update, "renew_lease"), nil, req, &resp,
		updateAccessToken(updateTokenStaticSource(token)), httpCallOptions{RetryPolicy: retryAllMethods}); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// PatchUpdateCheckpoint patches the checkpoint for the indicated update with the given contents.
func (pc *Client) PatchUpdateCheckpoint(
	ctx context.Context,
	update UpdateIdentifier,
	deployment *apitype.UntypedDeployment,
	token UpdateTokenSource,
) error {
	req := apitype.PatchUpdateCheckpointRequest{
		Version:    deployment.Version,
		Features:   deployment.Features,
		Deployment: deployment.Deployment,
	}

	// It is safe to retry this PATCH operation, because it is logically idempotent, since we send the entire
	// deployment instead of a set of changes to apply.
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "checkpoint"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryPolicy: retryAllMethods, GzipCompress: true})
}

// PatchUpdateCheckpointVerbatim is a variant of PatchUpdateCheckpoint that preserves JSON indentation of the
// UntypedDeployment transferred over the wire.
func (pc *Client) PatchUpdateCheckpointVerbatim(ctx context.Context, update UpdateIdentifier,
	sequenceNumber int, untypedDeploymentBytes json.RawMessage, deploymentVersion int, token UpdateTokenSource,
) error {
	req := apitype.PatchUpdateVerbatimCheckpointRequest{
		Version:           deploymentVersion,
		UntypedDeployment: untypedDeploymentBytes,
		SequenceNumber:    sequenceNumber,
	}

	reqPayload, err := marshalVerbatimCheckpointRequest(req)
	if err != nil {
		return err
	}

	// It is safe to retry this PATCH operation, because it is logically idempotent, since we send the entire
	// deployment instead of a set of changes to apply.
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "checkpointverbatim"), nil, reqPayload, nil,
		updateAccessToken(token), httpCallOptions{RetryPolicy: retryAllMethods, GzipCompress: true})
}

// PatchUpdateCheckpointDelta patches the checkpoint for the indicated update with the given contents, just like
// PatchUpdateCheckpoint. Unlike PatchUpdateCheckpoint, it uses a text diff-based protocol to conserve bandwidth on
// large stack states.
func (pc *Client) PatchUpdateCheckpointDelta(ctx context.Context, update UpdateIdentifier,
	sequenceNumber int, checkpointHash string, deploymentDelta json.RawMessage, deploymentVersion int,
	token UpdateTokenSource,
) error {
	req := apitype.PatchUpdateCheckpointDeltaRequest{
		Version:         deploymentVersion,
		CheckpointHash:  checkpointHash,
		SequenceNumber:  sequenceNumber,
		DeploymentDelta: deploymentDelta,
	}

	// It is safe to retry because SequenceNumber serves as an idempotency key.
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "checkpointdelta"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryPolicy: retryAllMethods, GzipCompress: true})
}

// SaveJournalEntry sends a single journal entry to the service. When we get a success response,
// the journal entry is guaranteed to be stored safely.
func (pc *Client) SaveJournalEntry(ctx context.Context, update UpdateIdentifier,
	entry apitype.JournalEntry, token UpdateTokenSource,
) error {
	return pc.SaveJournalEntries(ctx, update, []apitype.JournalEntry{entry}, token)
}

// SaveJournalEntries sends a single journal entry to the service. When we get a success response,
// all journal entries are guaranteed to be stored safely.
func (pc *Client) SaveJournalEntries(ctx context.Context, update UpdateIdentifier,
	entries []apitype.JournalEntry, token UpdateTokenSource,
) error {
	req := apitype.JournalEntries{
		Entries: entries,
	}
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "journalentries"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryPolicy: retryAllMethods, GzipCompress: true})
}

// CancelUpdate cancels the indicated update.
func (pc *Client) CancelUpdate(ctx context.Context, update UpdateIdentifier) error {
	// It is safe to retry this PATCH operation, because it is logically idempotent.
	return pc.restCallWithOptions(ctx, "POST", getUpdatePath(update, "cancel"), nil, nil, nil,
		httpCallOptions{RetryPolicy: retryAllMethods})
}

// CompleteUpdate completes the indicated update with the given status.
func (pc *Client) CompleteUpdate(ctx context.Context, update UpdateIdentifier, status apitype.UpdateStatus,
	token UpdateTokenSource,
) error {
	req := apitype.CompleteUpdateRequest{
		Status: status,
	}

	// It is safe to retry this PATCH operation, because it is logically idempotent.
	return pc.updateRESTCall(ctx, "POST", getUpdatePath(update, "complete"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryPolicy: retryAllMethods})
}

// GetUpdateEngineEventsOptions configures filtering and pagination for
// GetUpdateEngineEvents.
type GetUpdateEngineEventsOptions struct {
	// ContinuationToken, if non-nil, fetches the next page of events.
	ContinuationToken *string
	// EventTypes, if non-empty, restricts results to the listed engine event
	// type codes.
	EventTypes []string
	// URN, if non-empty, restricts results to events for the given resource URN.
	URN string
	// IncludeNonActivated, when true, includes events that have not yet been
	// marked as activated.
	IncludeNonActivated bool
}

// GetUpdateEngineEvents returns the engine events for an update.
func (pc *Client) GetUpdateEngineEvents(ctx context.Context, update UpdateIdentifier,
	opts GetUpdateEngineEventsOptions,
) (apitype.GetUpdateEventsResponse, error) {
	path := getUpdatePath(update, "events")

	query := url.Values{}
	if opts.ContinuationToken != nil {
		query.Set("continuationToken", *opts.ContinuationToken)
	}
	for _, t := range opts.EventTypes {
		query.Add("type", t)
	}
	if opts.URN != "" {
		query.Set("urn", opts.URN)
	}
	if opts.IncludeNonActivated {
		query.Set("include_non_activated", "true")
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var resp apitype.GetUpdateEventsResponse
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.GetUpdateEventsResponse{}, err
	}

	return resp, nil
}

// RecordEngineEvents posts a batch of engine events to the Pulumi service.
func (pc *Client) RecordEngineEvents(
	ctx context.Context, update UpdateIdentifier, batch apitype.EngineEventBatch, token UpdateTokenSource,
) error {
	callOpts := httpCallOptions{
		GzipCompress: true,
		RetryPolicy:  retryAllMethods,
	}
	return pc.updateRESTCall(
		ctx, "POST", getUpdatePath(update, "events/batch"),
		nil, batch, nil,
		updateAccessToken(token), callOpts)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (pc *Client) UpdateStackTags(
	ctx context.Context, stack StackIdentifier, tags map[apitype.StackTagName]string,
) error {
	// Validate stack tags.
	if err := validation.ValidateStackTags(tags); err != nil {
		return err
	}

	return pc.restCall(ctx, "PATCH", getStackPath(stack, "tags"), nil, tags, nil)
}

func (pc *Client) UpdateStackConfig(
	ctx context.Context, stack StackIdentifier, config *apitype.StackConfig,
) error {
	return pc.restCall(ctx, "PUT", getStackPath(stack, "config"), nil, config, nil)
}

// PatchStackDeploymentSettings merges the supplied patch into the stack's
// existing deployment settings. Wraps the `PatchDeploymentSettings` Pulumi
// Cloud REST endpoint (POST /api/stacks/{org}/{project}/{stack}/deployments/settings).
// For each property in the patch, the server starts with the current value,
// removes it if the patch specifies null, or merges the new non-null value
// with the existing one. Non-object properties are replaced entirely.
//
// Note we use json.RawMessage and not DeploymentSettings so that we can send
// partial objects (ie undefined values) or null values to delete settings.
func (pc *Client) PatchStackDeploymentSettings(ctx context.Context, stack StackIdentifier,
	patch json.RawMessage,
) error {
	return pc.restCall(ctx, http.MethodPost, getStackPath(stack, "deployments", "settings"), nil, patch, nil)
}

func (pc *Client) DestroyStackDeploymentSettings(ctx context.Context, stack StackIdentifier) error {
	return pc.restCall(ctx, "DELETE", getStackPath(stack, "deployments", "settings"), nil, nil, nil)
}

func (pc *Client) GetGHAppIntegration(
	ctx context.Context, stack StackIdentifier,
) (*apitype.GitHubAppIntegration, error) {
	var response apitype.GitHubAppIntegration

	err := pc.restCall(ctx, "GET", fmt.Sprintf("/api/console/orgs/%s/integrations/github-app",
		stack.Owner), nil, nil, &response)

	return &response, err
}

func (pc *Client) GetStackDeploymentSettings(ctx context.Context,
	stack StackIdentifier,
) (*apitype.DeploymentSettings, error) {
	var response apitype.DeploymentSettings

	err := pc.restCall(ctx, "GET", getStackPath(stack, "deployments", "settings"), nil, nil, &response)

	return &response, err
}

func getDeploymentPath(stack StackIdentifier, components ...string) string {
	prefix := fmt.Sprintf("/api/stacks/%s/%s/%s/deployments", stack.Owner, stack.Project, stack.Stack)
	escaped := make([]string, len(components))
	for i, c := range components {
		escaped[i] = url.PathEscape(c)
	}
	return path.Join(append([]string{prefix}, escaped...)...)
}

func (pc *Client) CreateDeployment(ctx context.Context, stack StackIdentifier,
	req apitype.CreateDeploymentRequest, deploymentInitiator string,
) (*apitype.CreateDeploymentResponse, error) {
	var resp apitype.CreateDeploymentResponse
	err := pc.restCallWithOptions(ctx, http.MethodPost, getDeploymentPath(stack), nil, req, &resp, httpCallOptions{
		Header: map[string][]string{
			"X-Pulumi-Deployment-Initiator": {deploymentInitiator},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating deployment failed: %w", err)
	}
	return &resp, nil
}

// GetDeployment retrieves a single deployment for a stack by its deployment
// ID. It wraps the `GetDeployment` Pulumi Cloud REST endpoint
// (GET /api/stacks/{org}/{project}/{stack}/deployments/{deploymentId}).
func (pc *Client) GetDeployment(
	ctx context.Context, stack StackIdentifier, id string,
) (apitype.GetDeploymentResponse, error) {
	var resp apitype.GetDeploymentResponse
	err := pc.restCall(ctx, http.MethodGet, getDeploymentPath(stack, id), nil, nil, &resp)
	if err != nil {
		return apitype.GetDeploymentResponse{}, fmt.Errorf("getting deployment %s failed: %w", id, err)
	}
	return resp, nil
}

// GetDeploymentByVersion retrieves a single deployment for a stack by its
// per-program version number. It wraps the Pulumi Cloud REST endpoint
// (GET /api/stacks/{org}/{project}/{stack}/deployments/version/{version}).
// The response is the same shape as GetDeployment; in particular `ID` is the
// deployment's UUID, which can be passed to the other per-deployment routes.
func (pc *Client) GetDeploymentByVersion(
	ctx context.Context, stack StackIdentifier, version string,
) (apitype.GetDeploymentResponse, error) {
	var resp apitype.GetDeploymentResponse
	err := pc.restCall(ctx, http.MethodGet, getDeploymentPath(stack, "version", version), nil, nil, &resp)
	if err != nil {
		return apitype.GetDeploymentResponse{}, fmt.Errorf("getting deployment by version %s failed: %w", version, err)
	}
	return resp, nil
}

// GetDeploymentLogsOptions configures the query parameters sent to the
// `GetDeploymentLogs` Pulumi Cloud REST endpoint. Pointer-typed fields encode
// "unset"; only fields the caller defines are serialized into the URL.
type GetDeploymentLogsOptions struct {
	Job               *int
	Step              *int
	Offset            *int
	Count             *int
	ContinuationToken string
}

// GetDeploymentLogs retrieves execution logs for a deployment. The endpoint
// supports two retrieval modes (see opts above): streaming mode (no job/step,
// optional ContinuationToken) and step mode (Job/Step required, Offset/Count
// optional).
func (pc *Client) GetDeploymentLogs(
	ctx context.Context, stack StackIdentifier, id string, opts GetDeploymentLogsOptions,
) (*apitype.DeploymentLogs, error) {
	q := url.Values{}
	if opts.Job != nil {
		q.Set("job", strconv.Itoa(*opts.Job))
	}
	if opts.Step != nil {
		q.Set("step", strconv.Itoa(*opts.Step))
	}
	if opts.Offset != nil {
		q.Set("offset", strconv.Itoa(*opts.Offset))
	}
	if opts.Count != nil {
		q.Set("count", strconv.Itoa(*opts.Count))
	}
	if opts.ContinuationToken != "" {
		q.Set("continuationToken", opts.ContinuationToken)
	}
	p := getDeploymentPath(stack, id, "logs")
	if encoded := q.Encode(); encoded != "" {
		p = p + "?" + encoded
	}
	var resp apitype.DeploymentLogs
	err := pc.restCall(ctx, http.MethodGet, p, nil, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s logs failed: %w", id, err)
	}
	return &resp, nil
}

func (pc *Client) GetDeploymentUpdates(ctx context.Context, stack StackIdentifier,
	id string,
) ([]apitype.GetDeploymentUpdatesUpdateInfo, error) {
	path := getDeploymentPath(stack, id, "updates")
	var resp []apitype.GetDeploymentUpdatesUpdateInfo
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s updates failed: %w", id, err)
	}
	return resp, nil
}

func (pc *Client) GetCapabilities(ctx context.Context) (*apitype.CapabilitiesResponse, error) {
	if pc.DisableCapabilityProbing {
		return &apitype.CapabilitiesResponse{}, nil
	}

	var resp apitype.CapabilitiesResponse
	err := pc.restCall(ctx, http.MethodGet, "/api/capabilities", nil, nil, &resp)
	if is404(err) {
		// The client continues to support legacy backends. They do not support /api/capabilities and are
		// assumed here to have no additional capabilities.
		return &apitype.CapabilitiesResponse{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying capabilities failed: %w", err)
	}
	return &resp, nil
}

func getSearchPath(orgName string) string {
	return fmt.Sprintf("/api/orgs/%s/search/resources", url.PathEscape(orgName))
}

func getNaturalLanguageSearchPath(orgName string) string {
	return fmt.Sprintf("/api/orgs/%s/search/resources/parse", url.PathEscape(orgName))
}

func getPulumiOrgSearchPath(baseURL string, orgName string) string {
	return fmt.Sprintf("%s/%s/resources", baseURL, url.PathEscape(orgName))
}

// Pulumi Cloud Search Functions
func (pc *Client) GetSearchQueryResults(
	ctx context.Context, orgName string, queryParams *apitype.PulumiQueryRequest, baseURL string,
) (*apitype.ResourceSearchResponse, error) {
	var resp apitype.ResourceSearchResponse
	err := pc.restCall(ctx, http.MethodGet, getSearchPath(orgName), queryParams, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("querying search failed: %w", err)
	}
	resp.URL = fmt.Sprintf("%s?query=%s", getPulumiOrgSearchPath(baseURL, orgName), url.QueryEscape(queryParams.Query))
	return &resp, nil
}

func (pc *Client) GetNaturalLanguageQueryResults(
	ctx context.Context, orgName string, queryString string,
) (*apitype.PulumiQueryResponse, error) {
	var resp apitype.PulumiQueryResponse
	queryParamObject := apitype.PulumiQueryRequest{
		Query: queryString,
	}
	err := pc.restCall(ctx, http.MethodGet, getNaturalLanguageSearchPath(orgName), queryParamObject, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("querying search failed: %w", err)
	}
	return &resp, nil
}

func is404(err error) bool {
	if err == nil {
		return false
	}
	var errResp *apitype.ErrorResponse
	if errors.As(err, &errResp) && errResp.Code == http.StatusNotFound {
		return true
	}
	return false
}

// SubmitAIPrompt sends the user's prompt to the Pulumi Service and streams back the response.
func (pc *Client) SubmitAIPrompt(ctx context.Context, requestBody any) (*http.Response, error) {
	url, err := url.Parse(pc.apiURL + getAIPromptPath())
	if err != nil {
		return nil, err
	}
	marshalledBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(marshalledBody))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("token %s", pc.apiToken))
	res, err := pc.restClient.HTTPClient().Do(request, retryAllMethods)
	return res, err
}

// SummarizeErrorWithNeo summarizes Pulumi Update output using the Copilot API
func (pc *Client) SummarizeErrorWithNeo(
	ctx context.Context,
	orgID string,
	content string,
	model string,
	maxSummaryLen int,
) (string, error) {
	request := createSummarizeUpdateRequest(content, orgID, model, maxSummaryLen, maxCopilotSummarizeUpdateContentLength)
	return pc.callCopilot(ctx, request)
}

func (pc *Client) ExplainPreviewWithNeo(
	ctx context.Context,
	orgID string,
	kind string,
	content string,
) (string, error) {
	request := createExplainPreviewRequest(content, orgID, kind, maxCopilotExplainPreviewContentLength)
	return pc.callCopilot(ctx, request)
}

// CreateNeoTaskOptions bundles the optional knobs on CreateNeoTask. The zero value
// accepts the server-side defaults for every field.
type CreateNeoTaskOptions struct {
	ToolExecutionMode   string
	ApprovalMode        NeoApprovalMode
	PermissionMode      NeoPermissionMode
	PlanMode            bool
	EnabledIntegrations *[]string
}

// CreateNeoTask creates a new Neo agent task via the Neo Tasks API. See
// CreateNeoTaskOptions for the available knobs.
func (pc *Client) CreateNeoTask(
	ctx context.Context,
	orgName string,
	content string,
	stackName string,
	projectName string,
	opts CreateNeoTaskOptions,
) (*NeoTaskResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, NeoCreateTaskTimeout)
	defer cancel()

	request := NeoTaskRequest{
		Message: NeoTaskMessage{
			Type:      "user_message",
			Content:   content,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		ToolExecutionMode:   opts.ToolExecutionMode,
		ApprovalMode:        opts.ApprovalMode,
		PermissionMode:      opts.PermissionMode,
		PlanMode:            opts.PlanMode,
		EnabledIntegrations: opts.EnabledIntegrations,
		Source:              NeoTaskSourceCLI,
	}
	// Only attach a stack entity when we actually have one — the backend rejects
	// entity_diff entries with empty name/project as "unable to access stack".
	if stackName != "" && projectName != "" {
		request.Message.EntityDiff = &NeoTaskEntityDiff{
			Add: []NeoTaskEntity{
				{Type: "stack", Name: stackName, Project: projectName},
			},
		}
	}

	path := fmt.Sprintf("/api/preview/agents/%s/tasks", orgName)
	var resp NeoTaskResponse
	if err := pc.restCall(ctx, http.MethodPost, path, nil, request, &resp); err != nil {
		return nil, fmt.Errorf("creating Neo task: %w", err)
	}

	return &resp, nil
}

// UpdateNeoTaskOptions bundles the fields a CLI session can change on a live task.
// Pointer fields let callers update one axis without resetting the other — matches
// the apitype.UpdateTaskRequest shape on the cloud side.
type UpdateNeoTaskOptions struct {
	ApprovalMode   *NeoApprovalMode   `json:"approvalMode,omitempty"`
	PermissionMode *NeoPermissionMode `json:"permissionMode,omitempty"`
}

// UpdateNeoTask PATCHes an existing Neo task with new approval / permission mode
// values. Used by the TUI's mid-session toggles (Ctrl+A / Ctrl+R) so the cloud
// ApprovalHandler picks up the change immediately. Fields left nil in opts are
// not sent — the server preserves the existing value.
func (pc *Client) UpdateNeoTask(
	ctx context.Context, orgName, taskID string, opts UpdateNeoTaskOptions,
) error {
	ctx, cancel := context.WithTimeout(ctx, NeoRequestTimeout)
	defer cancel()

	path := fmt.Sprintf("/api/preview/agents/%s/tasks/%s", orgName, taskID)
	if err := pc.restCall(ctx, http.MethodPatch, path, nil, opts, nil); err != nil {
		return fmt.Errorf("updating Neo task: %w", err)
	}
	return nil
}

// GetNeoTask fetches task metadata for an existing Neo task.
func (pc *Client) GetNeoTask(ctx context.Context, orgName, taskID string) (*NeoTask, error) {
	ctx, cancel := context.WithTimeout(ctx, NeoRequestTimeout)
	defer cancel()

	path := fmt.Sprintf("/api/preview/agents/%s/tasks/%s", orgName, taskID)
	var resp NeoTask
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return nil, fmt.Errorf("getting Neo task: %w", err)
	}
	return &resp, nil
}

// NeoStreamEvent is one item from a Neo task Server-Sent Events (SSE) stream. Exactly
// one of Data, KeepAlive, or Err is populated: Data carries an event payload, KeepAlive
// reports an SSE comment heartbeat, and Err carries a terminal stream error (after
// which no further values are sent before the channel closes). ID is the SSE `id:`
// field associated with the event (empty if absent); callers track it to send
// `Last-Event-ID` on reconnect so the server can replay missed events.
type NeoStreamEvent struct {
	Data      []byte
	ID        string
	KeepAlive bool
	Err       error
}

type neoTaskEventsResponse struct {
	Events            []apitype.AgentConsoleEvent `json:"events"`
	ContinuationToken *string                     `json:"continuationToken,omitempty"`
}

// GetNeoTaskEvents returns all currently recorded events for a Neo task and the
// newest event ID. Callers can pass the returned ID to StreamNeoTaskEvents as
// Last-Event-ID to attach from the live tail without replaying historical events.
func (pc *Client) GetNeoTaskEvents(
	ctx context.Context, orgName, taskID string,
) ([]apitype.AgentConsoleEvent, string, error) {
	ctx, cancel := context.WithTimeout(ctx, NeoRequestTimeout)
	defer cancel()

	var events []apitype.AgentConsoleEvent
	var lastEventID string
	var continuationToken string
	for {
		path := fmt.Sprintf("/api/preview/agents/%s/tasks/%s/events?pageSize=1000", orgName, taskID)
		if continuationToken != "" {
			path += "&continuationToken=" + url.QueryEscape(continuationToken)
		}

		var resp neoTaskEventsResponse
		if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
			return nil, "", fmt.Errorf("getting Neo task events: %w", err)
		}
		for _, event := range resp.Events {
			if event.ID != "" {
				lastEventID = event.ID
			}
		}
		events = append(events, resp.Events...)
		if resp.ContinuationToken == nil || *resp.ContinuationToken == "" {
			return events, lastEventID, nil
		}
		continuationToken = *resp.ContinuationToken
	}
}

// StreamNeoTaskEvents opens a Server-Sent Events (SSE) connection to the Neo task event
// stream and returns a channel of events. Each value carries either a raw event payload
// (the bytes following each `data:` line, joined for multi-line events) along with the
// `id:` of that event, a keep-alive marker for SSE comments, or a terminal stream error.
// The channel is closed when the stream ends or ctx is cancelled.
//
// If lastEventID is non-empty it is sent as the `Last-Event-ID` request header; the
// pulumi-service stream endpoint honors this and replays only events with sequence
// greater than the given ID, so a reconnect resumes losslessly.
//
// The endpoint is the SSE stream introduced in pulumi/pulumi-service#40132. This call
// does not impose its own timeout — callers should manage lifetime via ctx.
func (pc *Client) StreamNeoTaskEvents(
	ctx context.Context, orgName, taskID, lastEventID string,
) (<-chan NeoStreamEvent, error) {
	streamURL := pc.apiURL + fmt.Sprintf("/api/preview/agents/%s/tasks/%s/events/stream", orgName, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating Neo event stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("X-Pulumi-Source", "Pulumi CLI")
	apiToken, err := pc.apiToken.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching credentials: %w", err)
	}
	req.Header.Set("Authorization", "token "+apiToken)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	resp, err := pc.restClient.HTTPClient().Do(req, retryNone)
	if err != nil {
		return nil, fmt.Errorf("opening Neo event stream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("opening Neo event stream: HTTP %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	stream := make(chan NeoStreamEvent)

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		send := func(evt NeoStreamEvent) {
			select {
			case stream <- evt:
			case <-ctx.Done():
			}
		}

		// Tool payloads can be large, so give the scanner a generous max line size.
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

		var data bytes.Buffer
		// Per the SSE spec the "last event ID buffer" persists across events and is only
		// overwritten by a new `id:` line, so we don't reset eventID on flush.
		var eventID string
		flush := func() {
			if data.Len() == 0 {
				return
			}
			payload := bytes.Clone(data.Bytes())
			data.Reset()
			send(NeoStreamEvent{Data: payload, ID: eventID})
		}

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}
			if strings.HasPrefix(line, ":") {
				send(NeoStreamEvent{KeepAlive: true})
				continue
			}
			if chunk, ok := strings.CutPrefix(line, "data:"); ok {
				chunk = strings.TrimPrefix(chunk, " ")
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(chunk)
				continue
			}
			if chunk, ok := strings.CutPrefix(line, "id:"); ok {
				eventID = strings.TrimPrefix(chunk, " ")
				continue
			}
			// Other SSE fields (event:, retry:) are ignored — the server uses a single
			// event type and the JSON payload carries its own kind discriminator.
		}
		flush()
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			send(NeoStreamEvent{Err: fmt.Errorf("reading Neo event stream: %w", err)})
		}
	}()

	return stream, nil
}

// PostNeoTaskUserEvent posts an AgentUserEvent to a Neo task via the RespondToTask
// endpoint. The body must be a marshalable value matching one of the AgentUserEvent
// subtypes from pulumi-service's IDL (e.g. tool_result). The caller is responsible for
// setting the discriminator `type` field; the body is wrapped in the
// AgentRespondToTaskRequest envelope ({"event": <body>}) before being sent.
//
// Note: this is NOT the /events sub-resource — that one is reserved for the agent
// runtime posting AgentBackendEvents with an agent task token. User events go to the
// task root with a user PAT.
func (pc *Client) PostNeoTaskUserEvent(
	ctx context.Context, orgName, taskID string, body any,
) error {
	path := fmt.Sprintf("/api/preview/agents/%s/tasks/%s", orgName, taskID)
	return pc.restCall(ctx, http.MethodPost, path, nil, struct {
		Event any `json:"event"`
	}{Event: body}, nil)
}

func (pc *Client) callCopilot(ctx context.Context, requestBody any) (string, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, NeoRequestTimeout)
	defer cancel()

	url := pc.apiURL + "/api/ai/chat/preview"
	apiToken, err := pc.apiToken.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("fetching credentials: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Pulumi-Source", "Pulumi CLI")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+apiToken)

	resp, err := pc.restClient.HTTPClient().Do(req, retryAllMethods)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// Copilot API returns 204 No Content when it decided that it should not summarize the input.
		// This can happen when the input is too short or Copilot thinks it cannot make it any better.
		// In this case, we will not show the summary to the user. This is better than showing a useless summary.
		return "", nil
	}

	// Read the body first so we can use it for error reporting if needed
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		// For other error status codes, return the body as an error if it's readable, otherwise a generic error
		errorMsg := string(body)
		if errorMsg == "" {
			errorMsg = fmt.Sprintf("Copilot API returned error status: %d", resp.StatusCode)
		}
		return "", errors.New(errorMsg)
	}

	var copilotResp apitype.CopilotResponse
	if err := json.Unmarshal(body, &copilotResp); err != nil {
		return "", fmt.Errorf("unable to parse Copilot response: %s", string(body))
	}

	if copilotResp.Error != "" {
		return "", fmt.Errorf("copilot API error: %s\n%s", copilotResp.Error, copilotResp.Details)
	}

	return extractCopilotResponse(copilotResp)
}

func (pc *Client) PublishPackage(ctx context.Context, input apitype.PackagePublishOp) error {
	req := apitype.StartPackagePublishRequest{
		Version: input.Version.String(),
	}
	var resp apitype.StartPackagePublishResponse
	err := pc.restCall(ctx, "POST", publishPackagePath(input.Source, input.Publisher, input.Name), nil, req, &resp)
	if err != nil {
		return fmt.Errorf("publish package failed: %w", err)
	}

	uploadFile := func(url string, reader io.Reader, fileType string) error {
		putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, reader)
		if err != nil {
			return fmt.Errorf("failed to upload %s: %w", fileType, err)
		}
		for k, v := range resp.RequiredHeaders {
			putReq.Header.Add(k, v)
		}

		uploadResp, err := pc.restClient.HTTPClient().Do(putReq, retryAllMethods)
		if err != nil {
			return fmt.Errorf("failed to upload %s: %w", fileType, err)
		} else if uploadResp.StatusCode >= 400 {
			body, bodyErr := readBody(uploadResp)
			if bodyErr != nil {
				return fmt.Errorf("failed to upload %s: %s", fileType, uploadResp.Status)
			}
			return fmt.Errorf("failed to upload %s: %s - %s", fileType, uploadResp.Status, string(body))
		}

		return nil
	}

	err = uploadFile(resp.UploadURLs.Schema, input.Schema, "schema")
	if err != nil {
		return err
	}
	err = uploadFile(resp.UploadURLs.Index, input.Readme, "index")
	if err != nil {
		return err
	}
	if input.InstallDocs != nil {
		err = uploadFile(resp.UploadURLs.InstallationConfiguration, input.InstallDocs, "installation configuration")
		if err != nil {
			return err
		}
	}

	completeReq := apitype.CompletePackagePublishRequest{
		OperationID: resp.OperationID,
	}

	requestPath := completePackagePublishPath(input.Source, input.Publisher, input.Name, input.Version.String())
	err = pc.restCall(ctx, "POST", requestPath, nil, completeReq, nil)
	if err != nil {
		return fmt.Errorf("failed to complete package publishing operation %q: %w", resp.OperationID, err)
	}

	return nil
}

// StartTemplatePublish is a preview API, and should not be used without an approved EOL plan for
// deprecation.
func (pc *Client) StartTemplatePublish(
	ctx context.Context,
	source, publisher, name string,
	version semver.Version,
) (*StartTemplatePublishResponse, error) {
	req := StartTemplatePublishRequest{
		Version: version,
	}
	var resp StartTemplatePublishResponse
	err := pc.restCall(ctx, "POST", publishTemplatePath(source, publisher, name), nil, req, &resp)
	if err != nil {
		return nil, fmt.Errorf("start template publish failed: %w", err)
	}
	return &resp, nil
}

// CompleteTemplatePublish is a preview API, and should not be used without an approved EOL plan for
// deprecation.
func (pc *Client) CompleteTemplatePublish(
	ctx context.Context,
	source, publisher, name string,
	version semver.Version,
	operationID TemplatePublishOperationID,
) error {
	completeReq := PublishTemplateVersionCompleteRequest{
		OpID: operationID,
	}

	requestPath := completeTemplatePublishPath(source, publisher, name, version)
	err := pc.restCall(ctx, "POST", requestPath, nil, completeReq, nil)
	if err != nil {
		return fmt.Errorf("failed to complete template publishing operation %q: %w", operationID, err)
	}

	return nil
}

// PublishTemplate is a preview API, and should not be used without an approved EOL plan for
// deprecation.
func (pc *Client) PublishTemplate(ctx context.Context, input apitype.TemplatePublishOp) error {
	resp, err := pc.StartTemplatePublish(ctx, input.Source, input.Publisher, input.Name, input.Version)
	if err != nil {
		return fmt.Errorf("failed to start template publish: %w", err)
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, resp.UploadURLs.Archive, input.Archive)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/gzip")

	uploadResp, err := pc.restClient.HTTPClient().Do(putReq, retryAllMethods)
	if err != nil {
		return fmt.Errorf("failed to upload archive: %w", err)
	} else if uploadResp.StatusCode != http.StatusOK {
		body, bodyErr := readBody(uploadResp)
		if bodyErr != nil {
			return fmt.Errorf("failed to upload archive: %s", uploadResp.Status)
		}
		return fmt.Errorf("failed to upload archive: %s - %s", uploadResp.Status, string(body))
	}

	err = pc.CompleteTemplatePublish(ctx, input.Source, input.Publisher, input.Name, input.Version, resp.OperationID)
	if err != nil {
		return fmt.Errorf("failed to complete template publish: %w", err)
	}

	return nil
}

func (pc *Client) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	v := "latest"
	if version != nil {
		v = version.String()
	}
	url := fmt.Sprintf("/api/registry/packages/%s/%s/%s/versions/%s", source, publisher, name, v)
	var resp apitype.PackageMetadata
	err := pc.restCall(ctx, "GET", url, nil, nil, &resp)
	return resp, err
}

// DeletePackageVersion deletes a specific version of a package from the registry.
func (pc *Client) DeletePackageVersion(
	ctx context.Context, source, publisher, name string, version semver.Version,
) error {
	url := deletePackageVersionPath(source, publisher, name, version.String())
	err := pc.restCall(ctx, "DELETE", url, nil, nil, nil)
	return err
}

// RawCall issues an arbitrary Pulumi API request and returns the raw
// *http.Response. Unlike the typed Call methods, RawCall does not
// deserialize the response body and does not classify 4xx/5xx into typed
// errors; the caller inspects status, headers, and body. Gzip-encoded
// responses are transparently decompressed; the Content-Encoding and
// Content-Length response headers are left intact so the caller can still
// see what the server sent.
func (pc *Client) RawCall(
	ctx context.Context,
	method, path string,
	query url.Values,
	body io.Reader,
	header http.Header,
	gzipCompressBody bool,
) (*http.Response, error) {
	fullPath := path
	if len(query) > 0 {
		fullPath += "?" + query.Encode()
	}

	var reqObj any
	if body != nil {
		bodyBytes, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		if len(bodyBytes) > 0 {
			// json.RawMessage is sent verbatim by the rest-call path — works
			// for non-JSON content too
			reqObj = json.RawMessage(bodyBytes)
		}
	}

	var resp *http.Response
	err := pc.restCallWithOptions(ctx, method, fullPath, nil, reqObj, &resp, httpCallOptions{
		SkipDecodeErrors: true,
		Header:           header,
		GzipCompress:     gzipCompressBody,
	})
	if err != nil {
		return nil, err
	}

	if encs, ok := resp.Header["Content-Encoding"]; ok && len(encs) == 1 &&
		(encs[0] == "gzip" || encs[0] == "x-gzip") {
		gr, gzerr := gzip.NewReader(resp.Body)
		if gzerr != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decompressing gzipped response: %w", gzerr)
		}
		resp.Body = &gzipReadCloser{gzip: gr, body: resp.Body}
	}
	return resp, nil
}

// GetCloudAPISpec fetches the Pulumi Cloud OpenAPI document as raw bytes.
func (pc *Client) GetCloudAPISpec(ctx context.Context) ([]byte, error) {
	var body []byte
	err := pc.restCallWithOptions(ctx, "GET", "/api/openapi/pulumi-spec.json", nil, nil, &body,
		httpCallOptions{Header: http.Header{"Accept": []string{"application/json"}}})
	return body, err
}

func (pc *Client) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	v := "latest"
	if version != nil {
		v = version.String()
	}
	url := fmt.Sprintf("/api/registry/templates/%s/%s/%s/versions/%s", source, publisher, name, v)
	var resp apitype.TemplateMetadata
	err := pc.restCall(ctx, "GET", url, nil, nil, &resp)
	return resp, err
}

// DownloadTemplate takes a downloadURL, which is a full URL (not a path). The URL can be
// one of two situations:
//
// - An URL for the client: `api.pulumi.com/api/orgs/{org}/template/download?name={template-name}`
// - A foreign URL: `api.github.com/blobs/4328791917840`
//
// We want to use the full method receiver with configured credentials if and only if we
// are targeting the correct URL. If we are targeting another URL, we should use a fresh
// client without tokens.
func (pc *Client) DownloadTemplate(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
	// Presigned URLs need to be handled differently to avoid signature mismatches due to e.g. headers.
	// See https://docs.aws.amazon.com/prescriptive-guidance/latest/presigned-url-best-practices/identifying-requests.html
	isPresignedURL := strings.Contains(downloadURL, "X-Amz-Expires=")
	if after, ok := strings.CutPrefix(downloadURL, pc.apiURL); ok {
		downloadURL = after
	} else if isPresignedURL {
		return pc.downloadWithRawClient(ctx, downloadURL)
	} else {
		// Set pc to the new client. This only sets the local variable. It is very
		// different from *pc = *NewClient().
		pc = NewClient(downloadURL, "", true, pc.diag)
		downloadURL = ""
	}

	var bytes io.ReadCloser
	header := make(http.Header, 1)
	header.Add("Accept", "application/x-tar")

	err := pc.restCallWithOptions(ctx, "GET", downloadURL, nil, nil, &bytes, httpCallOptions{
		Header: header,
	})
	return bytes, err
}

func (pc *Client) downloadWithRawClient(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pc.restClient.HTTPClient().Do(req, retryAllMethods)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return bodyIntoReader(resp)
}

func (pc *Client) ListPackages(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
	url := "/api/registry/packages?limit=499"
	if name != nil {
		url += "&name=" + *name
	}

	var continuationToken *string
	return func(f func(apitype.PackageMetadata, error) bool) {
		for {
			queryURL := url
			if continuationToken != nil {
				queryURL += "&continuationToken=" + *continuationToken
			}
			var resp apitype.ListPackagesResponse
			err := pc.restCall(ctx, "GET", queryURL, nil, nil, &resp)
			if err != nil {
				f(apitype.PackageMetadata{}, err)
				return
			}
			for _, v := range resp.Packages {
				if !f(v, nil) {
					return
				}
			}
			continuationToken = resp.ContinuationToken
			if continuationToken == nil {
				return
			}
		}
	}
}

func (pc *Client) ListTemplates(
	ctx context.Context, opts registry.ListTemplatesOptions,
) iter.Seq2[apitype.TemplateMetadata, error] {
	query := url.Values{}
	query.Set("limit", "499")
	if opts.Name != "" {
		query.Set("name", opts.Name)
	}
	if opts.Org != "" {
		query.Set("orgLogin", opts.Org)
	}
	if opts.Search != "" {
		query.Set("search", opts.Search)
	}

	var continuationToken *string
	return func(f func(apitype.TemplateMetadata, error) bool) {
		for {
			pageQuery := query
			if continuationToken != nil {
				// Clone so we don't mutate the captured map between iterations.
				pageQuery = url.Values{}
				for k, v := range query {
					pageQuery[k] = v
				}
				pageQuery.Set("continuationToken", *continuationToken)
			}
			var resp apitype.ListTemplatesResponse
			err := pc.restCall(ctx, "GET", "/api/registry/templates?"+pageQuery.Encode(), nil, nil, &resp)
			if err != nil {
				f(apitype.TemplateMetadata{}, err)
				return
			}
			for _, v := range resp.Templates {
				if !f(v, nil) {
					return
				}
			}
			continuationToken = resp.ContinuationToken
			if continuationToken == nil {
				return
			}
		}
	}
}

// GetInsightsResource fetches a single resource discovered by Pulumi Insights.
//
// The `accountName` and `resourceTypeAndId` path parameters are double-decoded
// on the service side, so we double-URL-encode them here to preserve any
// embedded `/`, `:`, or `::` characters intact through the full decode chain.
// `resourceTypeAndId` is the colon-separated `<type>::<id>` identifier as
// described by the OpenAPI spec for the ReadResource operation.
func (pc *Client) GetInsightsResource(
	ctx context.Context, org, account, resourceTypeAndId string,
) (apitype.InsightsResourceWithVersion, error) {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s/resources/%s",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
		url.PathEscape(url.PathEscape(resourceTypeAndId)),
	)
	var resp apitype.InsightsResourceWithVersion
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.InsightsResourceWithVersion{}, err
	}
	return resp, nil
}

// ScanInsightsAccount starts a resource discovery scan for an Insights account.
// For parent accounts, the server fans the scan out across child accounts.
//
// The `accountName` path parameter is double-decoded on the service side, so we
// double-URL-encode it here to preserve any embedded `/` intact through the
// full decode chain.
//
// The service currently returns 204 No Content on success (no body), even
// though the OpenAPI spec advertises a [apitype.InsightsScanResponse]. We
// surface a zero-value response in that case; when the server starts returning
// the documented JSON, the decode path picks it up automatically.
func (pc *Client) ScanInsightsAccount(
	ctx context.Context, org, account string, req apitype.InsightsScanRequest,
) (apitype.InsightsScanResponse, error) {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s/scan",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
	)
	// Read the body into a byte slice first so an empty 204 doesn't fall into
	// `json.Unmarshal` and trip over "unexpected end of JSON input".
	var raw []byte
	if err := pc.restCall(ctx, "POST", path, nil, req, &raw); err != nil {
		return apitype.InsightsScanResponse{}, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return apitype.InsightsScanResponse{}, nil
	}
	var resp apitype.InsightsScanResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return apitype.InsightsScanResponse{}, fmt.Errorf("decoding scan response: %w", err)
	}
	return resp, nil
}

// SearchInsightsResources runs a resource search against the v2 endpoint
// (`GetOrgResourceSearchV2Query`).
//
// Zero-valued fields on params are omitted from the query string so the server
// can apply its own defaults — see [apitype.InsightsResourceSearchParams] for
// the per-field semantics.
func (pc *Client) SearchInsightsResources(
	ctx context.Context, org string, params apitype.InsightsResourceSearchParams,
) (apitype.InsightsResourceSearchResponse, error) {
	path := fmt.Sprintf("/api/orgs/%s/search/resourcesv2", url.PathEscape(org))
	var resp apitype.InsightsResourceSearchResponse
	if err := pc.restCall(ctx, "GET", path, &params, nil, &resp); err != nil {
		return apitype.InsightsResourceSearchResponse{}, err
	}
	return resp, nil
}

// ListInsightsAccounts fetches a single page of Pulumi Insights accounts for an
// organization. The caller is responsible for following the NextToken cursor
// across pages; zero-valued fields on params are omitted from the query string
// so the service applies its own defaults.
func (pc *Client) ListInsightsAccounts(
	ctx context.Context, org string, params apitype.ListInsightsAccountsParams,
) (apitype.ListInsightsAccountsResponse, error) {
	path := fmt.Sprintf("/api/preview/insights/%s/accounts", url.PathEscape(org))
	var resp apitype.ListInsightsAccountsResponse
	if err := pc.restCall(ctx, "GET", path, &params, nil, &resp); err != nil {
		return apitype.ListInsightsAccountsResponse{}, err
	}
	return resp, nil
}

// GetInsightsScan fetches the full workflow run for a single Pulumi Insights
// scan, including jobs and steps. The list endpoint (ListInsightsAccountScans)
// only returns the per-scan summary; this is the only way to see the run's
// jobs/steps and per-step status.
//
// `account` and `scanId` are both path parameters; the service double-decodes
// `account`, so we double-URL-encode it — same convention as GetInsightsAccount
// / CreateInsightsAccount / GetInsightsResource.
func (pc *Client) GetInsightsScan(
	ctx context.Context, org, account, scanID string,
) (apitype.InsightsScanResponse, error) {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s/scans/%s",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
		url.PathEscape(scanID),
	)
	var resp apitype.InsightsScanResponse
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.InsightsScanResponse{}, err
	}
	return resp, nil
}

// ListInsightsAccountScans fetches a page of recent scans for an Insights account.
// For parent accounts the endpoint returns scans across all child accounts,
// so it is the recommended way to discover scan IDs to feed into GetInsightsScanLogs.
//
// The `accountName` path parameter is double-decoded on the service side
func (pc *Client) ListInsightsAccountScans(
	ctx context.Context, org, account string, params apitype.ListInsightsAccountScansParams,
) (apitype.ListInsightsAccountScansResponse, error) {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s/scans",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
	)
	var resp apitype.ListInsightsAccountScansResponse
	if err := pc.restCall(ctx, "GET", path, &params, nil, &resp); err != nil {
		return apitype.ListInsightsAccountScansResponse{}, err
	}
	return resp, nil
}

// CreateInsightsAccount creates a new Pulumi Insights account.
//
// The `accountName` path parameter is double-decoded on the service side, so
// we double-URL-encode it here — matching the convention already used by
// GetInsightsResource. The endpoint returns 204 No Content on success; no
// response body is parsed.
func (pc *Client) CreateInsightsAccount(
	ctx context.Context, org, account string, req apitype.CreateInsightsAccountRequest,
) error {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
	)
	return pc.restCall(ctx, "POST", path, nil, req, nil)
}

// GetInsightsAccount fetches the details of a Pulumi Insights account. The
// `accountName` path parameter is double-decoded on the service side, so we
// double-URL-encode it here — same convention as CreateInsightsAccount and
// GetInsightsResource.
func (pc *Client) GetInsightsAccount(
	ctx context.Context, org, account string,
) (apitype.InsightsAccount, error) {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
	)
	var resp apitype.InsightsAccount
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.InsightsAccount{}, err
	}
	return resp, nil
}

// ListESCEnvironments fetches a page of ESC environments visible to the
// caller in the named organization. Mirrors the endpoint exposed by the ESC
// CLI (`pulumi esc ls`) so the Pulumi CLI can offer the same picker without
// pulling in the esc-cli library.
//
// The endpoint paginates with an opaque continuation token; nextToken is
// empty on the last page.
func (pc *Client) ListESCEnvironments(
	ctx context.Context, org, continuationToken string,
) ([]apitype.ESCEnvironment, string, error) {
	queryObj := struct {
		ContinuationToken string `url:"continuationToken,omitempty"`
	}{ContinuationToken: continuationToken}

	path := "/api/esc/environments/" + url.PathEscape(org)
	var resp apitype.ListESCEnvironmentsResponse
	if err := pc.restCall(ctx, "GET", path, queryObj, nil, &resp); err != nil {
		return nil, "", err
	}
	return resp.Environments, resp.NextToken, nil
}

// ListStackDeploymentsOptions are the optional query parameters accepted by
// ListStackDeployments. Zero values mean "let the server pick the default":
// Page < 1 → 1, PageSize ≤ 0 → 10 (server-side cap 100), Sort "" → server's
// default, Asc false → descending order.
type ListStackDeploymentsOptions struct {
	Page     int64
	PageSize int64
	Sort     string
	Asc      bool
}

// ListStackDeployments returns a paginated list of deployments for the given
// stack, wrapping the ListStackDeploymentsHandlerV2 endpoint.
func (pc *Client) ListStackDeployments(
	ctx context.Context, stack StackIdentifier, opts ListStackDeploymentsOptions,
) (apitype.ListDeploymentResponseV2, error) {
	query := url.Values{}
	if opts.Page > 0 {
		query.Set("page", strconv.FormatInt(opts.Page, 10))
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", strconv.FormatInt(opts.PageSize, 10))
	}
	if opts.Sort != "" {
		query.Set("sort", opts.Sort)
	}
	if opts.Asc {
		// Only set `asc` when it's non-default — the server treats the absence
		// of the flag as descending, matching our zero value.
		query.Set("asc", "true")
	}

	path := getStackPath(stack, "deployments")
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var resp apitype.ListDeploymentResponseV2
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
		return apitype.ListDeploymentResponseV2{}, err
	}
	return resp, nil
}

// GetOrgUsageSummary fetches the Resources Under Management (RUM) and
// Resource Hours Under Management (RHUM) summary for an organization, wrapping
// the GetUsageSummaryResourceHours endpoint.
//
// Zero-valued fields on params are omitted from the query string so the server
// can apply its own defaults — see [apitype.OrgUsageSummaryParams] for the
// per-field semantics.
func (pc *Client) GetOrgUsageSummary(
	ctx context.Context, org string, params apitype.OrgUsageSummaryParams,
) (apitype.OrgUsageSummaryResponse, error) {
	path := fmt.Sprintf("/api/orgs/%s/resources/summary", url.PathEscape(org))
	var resp apitype.OrgUsageSummaryResponse
	if err := pc.restCall(ctx, "GET", path, &params, nil, &resp); err != nil {
		return apitype.OrgUsageSummaryResponse{}, err
	}
	return resp, nil
}

// CancelStackDeployment requests cancellation of an in-progress Pulumi
// Deployments execution. Wraps the CancelDeployment endpoint.
//
// The endpoint is fire-and-forget: a 200 OK signals that the request was
// accepted, not that the deployment has finished tearing down. The server
// returns 404 when the deployment is not known. We treat the call as
// idempotent enough to retry on transient transport failures — the worst case
// is a redundant cancel against an already-canceling deployment.
func (pc *Client) CancelStackDeployment(
	ctx context.Context, stack StackIdentifier, deploymentID string,
) error {
	path := getStackPath(stack, "deployments", deploymentID, "cancel")
	return pc.restCallWithOptions(ctx, "POST", path, nil, nil, nil,
		httpCallOptions{RetryPolicy: retryAllMethods})
}

// GetInsightsScanLogs wraps the GetScanLogs endpoint. See
// [apitype.InsightsScanLogsParams] for mode and per-field semantics.
//
// `accountName` is double-decoded server-side, hence the double encoding here.
func (pc *Client) GetInsightsScanLogs(
	ctx context.Context, org, account, scanID string, params apitype.InsightsScanLogsParams,
) (apitype.InsightsScanLogs, error) {
	path := fmt.Sprintf(
		"/api/preview/insights/%s/accounts/%s/scans/%s/logs",
		url.PathEscape(org),
		url.PathEscape(url.PathEscape(account)),
		url.PathEscape(scanID),
	)
	var resp apitype.InsightsScanLogs
	if err := pc.restCall(ctx, "GET", path, &params, nil, &resp); err != nil {
		return apitype.InsightsScanLogs{}, err
	}
	return resp, nil
}

// CreateLogEncryptionSession creates a new log encryption session via the
// Pulumi Cloud API. The service generates a session key and returns it
// along with a session ID that can be used to identify the key later.
func (pc *Client) CreateLogEncryptionSession(
	ctx context.Context,
	req apitype.LogEncryptionSessionInitRequest,
) (*apitype.LogEncryptionSessionInitResponse, error) {
	var resp apitype.LogEncryptionSessionInitResponse
	if err := pc.restCall(ctx, http.MethodPost, "/api/log-encryption-session/init", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
