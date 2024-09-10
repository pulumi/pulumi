// Copyright 2023, Pulumi Corporation.
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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/version"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const etagHeader = "ETag"
const revisionHeader = "Pulumi-ESC-Revision"
const DefaultProject = "default"

type CheckYAMLOption struct {
	ShowSecrets bool
}

// Client provides a slim wrapper around the Pulumi HTTP/REST API.
//
// NOTE: this is not considered a public API, and we reserve the right to make breaking changes, including adding
// parameters, removing methods, changing types, etc.
//
// However, there is currently a cyclic dependency between the Pulumi CLI and the ESC CLI that causes breaking changes
// to any part of the client API to break the ESC CLI build. So we're limited to non-breaking changes, including adding
// variadic args or adding additional methods.
type Client interface {
	// Insecure returns true if this client is insecure (i.e. has TLS disabled).
	Insecure() bool

	// URL returns the URL of the API endpoint this client interacts with
	URL() string

	// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
	GetPulumiAccountDetails(ctx context.Context) (string, []string, *workspace.TokenInformation, error)

	// GetRevisionNumber returns the revision number for version.
	GetRevisionNumber(ctx context.Context, orgName, projectName, envName, version string) (int, error)

	// ListEnvironments lists all environments in the given org that are accessible to the calling user.
	//
	// Each call to ListEnvironments returns a page of results and a continuation token. If there are no
	// more results, the continuation token will be empty. Otherwise, the continuattion token should be
	// passed to the next call to ListEnvironments to fetch the next page of results.
	ListEnvironments(
		ctx context.Context,
		orgName string,
		continuationToken string,
	) (environments []OrgEnvironment, nextToken string, err error)

	// Deprecated: Use CreateEnvironmentWithProject instead
	CreateEnvironment(ctx context.Context, orgName, envName string) error

	// CreateEnvironment creates an environment named projectName/envName in orgName.
	CreateEnvironmentWithProject(ctx context.Context, orgName, projectName, envName string) error

	// CloneEnvironment clones an source environment into a new destination environment.
	CloneEnvironment(ctx context.Context, orgName, srcEnvProject, srcEnvName string, destEnv CloneEnvironmentRequest) error

	// GetEnvironment returns the YAML + ETag for the environment envName in org orgName. If decrypt is
	// true, any { fn::secret: { ciphertext: "..." } } constructs in the definition will be decrypted and
	// replaced with { fn::secret: "plaintext" }.
	//
	// The etag returned by GetEnvironment can be passed to UpdateEnvironment in order to avoid RMW issues
	// when editing envirionments.
	GetEnvironment(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		version string,
		decrypt bool,
	) (yaml []byte, etag string, revision int, err error)

	// UpdateEnvironmentWithRevision updates the YAML for the environment envName in org orgName.
	//
	// If the new environment definition contains errors, the update will fail with diagnostics.
	//
	// If etag is not the empty string and the environment's current etag does not match the provided etag
	// (i.e. because a different entity has called UpdateEnvironment), the update will fail with a 409
	// error.
	UpdateEnvironmentWithRevision(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		yaml []byte,
		etag string,
	) ([]EnvironmentDiagnostic, int, error)

	// Deprecated: Use UpdateEnvironmentWithProject instead
	UpdateEnvironment(
		ctx context.Context,
		orgName string,
		envName string,
		yaml []byte,
		etag string,
	) ([]EnvironmentDiagnostic, error)
	// This method has a legacy signature, please use UpdateEnvironmentWithRevision instead
	// Remove this method once circular dependency between esc and pulumi/pulumi is resolved
	UpdateEnvironmentWithProject(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		yaml []byte,
		etag string,
	) ([]EnvironmentDiagnostic, error)

	// DeleteEnvironment deletes the environment envName in org orgName.
	DeleteEnvironment(ctx context.Context, orgName, projectName, envName string) error

	// OpenEnvironment evaluates the environment envName in org orgName and returns the ID of the opened
	// environment. The opened environment will be available for the indicated duration, after which it
	// will expire.
	//
	// If the environment contains errors, the open will fail with diagnostics.
	OpenEnvironment(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		version string,
		duration time.Duration,
	) (string, []EnvironmentDiagnostic, error)

	// CheckYAMLEnvironment checks the given environment YAML for errors within the context of org orgName.
	//
	// This call returns the checked environment's AST, values, schema, and any diagnostics issued by the
	// evaluator.
	CheckYAMLEnvironment(
		ctx context.Context,
		orgName string,
		yaml []byte,
		opts ...CheckYAMLOption,
	) (*esc.Environment, []EnvironmentDiagnostic, error)

	// OpenYAMLEnvironment evaluates the given environment YAML within the context of org orgName and
	// returns the ID of the opened environment. The opened environment will be available for the indicated
	// duration, after which it will expire.
	//
	// If the environment contains errors, the open will fail with diagnostics.
	OpenYAMLEnvironment(
		ctx context.Context,
		orgName string,
		yaml []byte,
		duration time.Duration,
	) (string, []EnvironmentDiagnostic, error)

	// Deprecated: Use GetOpenEnvironmentWithProject instead
	GetOpenEnvironment(ctx context.Context, orgName, envName, openEnvID string) (*esc.Environment, error)
	// GetOpenEnvironmentWithProject returns the AST, values, and schema for the open environment with ID openEnvID in
	// environment envName and org orgName.
	GetOpenEnvironmentWithProject(ctx context.Context, orgName, projectName, envName, openEnvID string) (*esc.Environment, error)

	// GetAnonymousOpenEnvironment returns the AST, values, and schema for the open environment with ID openEnvID in
	// an anonymous environment.
	GetAnonymousOpenEnvironment(ctx context.Context, orgName, openEnvID string) (*esc.Environment, error)

	// GetOpenProperty returns the value of a single property in the open environment with ID openEnvID in
	// environment envName and org orgName.
	//
	// The property parameter is a Pulumi property path. Property paths may contain dotted accessors and
	// numeric or string subscripts. For example:
	//
	//     foo.bar[0]["baz"]
	//     aws.login
	//     environmentVariables["AWS_ACCESS_KEY_ID"]
	//
	GetOpenProperty(ctx context.Context, orgName, projectName, envName, openEnvID, property string) (*esc.Value, error)

	// GetOpenProperty returns the value of a single property in the open environment with ID openEnvID in
	// an anonymous environment.
	GetAnonymousOpenProperty(ctx context.Context, orgName, openEnvID, property string) (*esc.Value, error)

	// ListEnvironmentTags lists the tags for the given environment.
	ListEnvironmentTags(
		ctx context.Context,
		orgName, projectName, envName string,
		options ListEnvironmentTagsOptions,
	) ([]*EnvironmentTag, string, error)

	// CreateEnvironmentTag creates and applies a tag to the given environment.
	CreateEnvironmentTag(
		ctx context.Context,
		orgName, projectName, envName, key, value string,
	) (*EnvironmentTag, error)

	// GetEnvironmentTag returns a tag with the specified name for the given environment.
	GetEnvironmentTag(
		ctx context.Context,
		orgName, projectName, envName, key string,
	) (*EnvironmentTag, error)

	// UpdateEnvironmentTag updates a specified environment tag with a new key / value.
	UpdateEnvironmentTag(
		ctx context.Context,
		orgName, projectName, envName, currentKey, currentValue, newKey, newValue string,
	) (*EnvironmentTag, error)

	// DeleteEnvironmentTag deletes a specified tag on an environment.
	DeleteEnvironmentTag(ctx context.Context, orgName, projectName, envName, tagName string) error

	// GetEnvironmentRevision returns a description of the given revision.
	GetEnvironmentRevision(ctx context.Context, orgName, projectName, envName string, revision int) (*EnvironmentRevision, error)

	// ListEnvironmentRevisions returns a list of revisions to the named environments in reverse order by
	// revision number. The revision at which to start and the number of revisions to return are
	// configurable via the options parameter.
	ListEnvironmentRevisions(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		options ListEnvironmentRevisionsOptions,
	) ([]EnvironmentRevision, error)

	// RetractEnvironmentRevision retracts a specific revision of an environment.
	RetractEnvironmentRevision(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		version string,
		replacement *int,
		reason string,
	) error

	// CreateEnvironmentRevisionTag creates a new revision tag with the given name.
	CreateEnvironmentRevisionTag(ctx context.Context, orgName, projectName, envName, tagName string, revision *int) error

	// GetEnvironmentRevisionTag returns a description of the given revision tag.
	GetEnvironmentRevisionTag(ctx context.Context, orgName, projectName, envName, tagName string) (*EnvironmentRevisionTag, error)

	// UpdateEnvironmentRevisionTag updates the revision tag with the given name.
	UpdateEnvironmentRevisionTag(ctx context.Context, orgName, projectName, envName, tagName string, revision *int) error

	// DeleteEnvironmentRevisionTag deletes the revision tag with the given name.
	DeleteEnvironmentRevisionTag(ctx context.Context, orgName, projectName, envName, tagName string) error

	// ListEnvironmentRevisionTags lists the revision tags for the given environment.
	ListEnvironmentRevisionTags(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
		options ListEnvironmentRevisionTagsOptions,
	) ([]EnvironmentRevisionTag, error)

	// EnvironmentExists checks if the specified environment exists.
	EnvironmentExists(
		ctx context.Context,
		orgName string,
		projectName string,
		envName string,
	) (exists bool, err error)
}

type client struct {
	apiURL     string
	apiToken   string
	apiUser    string
	apiOrgs    []string
	tokenInfo  *workspace.TokenInformation // might be nil if running against old services
	insecure   bool
	userAgent  string
	httpClient *http.Client
}

func newHTTPClient(insecure bool) *http.Client {
	if insecure {
		return &http.Client{
			Transport: &http.Transport{
				//nolint:gosec // The user has explicitly opted into setting this
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return http.DefaultClient
}

// newClient creates a new ESC API client with the given user agent, URL, API token, and HTTP client.
func newClient(userAgent, apiURL, apiToken string, httpClient *http.Client) *client {
	return &client{
		apiURL:     apiURL,
		apiToken:   apiToken,
		userAgent:  userAgent,
		httpClient: httpClient,
	}
}

// Insecure returns true if this client is insecure (i.e. has TLS disabled).
func (pc *client) Insecure() bool {
	return pc.insecure
}

// New creates a new Pulumi API client with the given URL and API token.
func NewDefaultClient(apiToken string) Client {
	userAgent := fmt.Sprintf("esc-sdk/1 (%s; %s)", version.Version, runtime.GOOS)
	return New(userAgent, "https://api.pulumi.com", apiToken, false)
}

// New creates a new Pulumi API client with the given URL and API token.
func New(userAgent, apiURL, apiToken string, insecure bool) Client {
	return newClient(userAgent, apiURL, apiToken, newHTTPClient(insecure))
}

// URL returns the URL of the API endpoint this client interacts with
func (pc *client) URL() string {
	return pc.apiURL
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

// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
func (pc *client) GetPulumiAccountDetails(ctx context.Context) (string, []string, *workspace.TokenInformation, error) {
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

// resolveEnvironmentPath resolves an environment and revision or tag to its API path.
//
// If version begins with a digit, it is treated as a revision number. Otherwise, it is treated as a tag.
// If no revision or tag is present, the "latest" tag is used.
func (pc *client) resolveEnvironmentPath(orgName, projectName, envName, version string) (string, error) {
	if version == "" {
		return fmt.Sprintf("/api/esc/environments/%v/%v/%v", orgName, projectName, envName), nil
	}
	return fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/%v", orgName, projectName, envName, version), nil
}

func (pc *client) GetRevisionNumber(ctx context.Context, orgName, projectName, envName, version string) (int, error) {
	if version == "" {
		version = "latest"
	} else if version[0] >= '0' && version[0] <= '9' {
		rev, err := strconv.ParseInt(version, 10, 0)
		if err != nil {
			return 0, fmt.Errorf("invalid revision number %q", version)
		}
		return int(rev), nil
	}

	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/tags/%v", orgName, projectName, envName, version)

	var resp EnvironmentRevisionTag
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return 0, fmt.Errorf("resolving tag %q: %w", version, err)
	}
	return resp.Revision, nil
}

func (pc *client) ListEnvironments(
	ctx context.Context,
	orgName string,
	continuationToken string,
) ([]OrgEnvironment, string, error) {
	queryObj := struct {
		ContinuationToken string `url:"continuationToken,omitempty"`
		Organization      string `url:"organization,omitempty"`
	}{
		ContinuationToken: continuationToken,
		Organization:      orgName,
	}

	var resp ListEnvironmentsResponse
	err := pc.restCall(ctx, http.MethodGet, "/api/esc/environments", queryObj, nil, &resp)
	if err != nil {
		return nil, "", err
	}
	return resp.Environments, resp.NextToken, nil
}

// Deprecated: Use CreateEnvironmentWithProject instead
func (pc *client) CreateEnvironment(ctx context.Context, orgName, envName string) error {
	return pc.CreateEnvironmentWithProject(ctx, orgName, DefaultProject, envName)
}

// CreateEnvironmentWithProject creates an environment named envName in org orgName and project projectName.
func (pc *client) CreateEnvironmentWithProject(ctx context.Context, orgName, projectName, envName string) error {
	req := struct {
		Project string `json:"project"`
		Name    string `json:"name"`
	}{
		Project: projectName,
		Name:    envName,
	}

	path := fmt.Sprintf("/api/esc/environments/%v", orgName)
	return pc.restCall(ctx, http.MethodPost, path, nil, req, nil)
}

func (pc *client) CloneEnvironment(
	ctx context.Context,
	orgName, srcEnvProject, srcEnvName string,
	destEnv CloneEnvironmentRequest,
) error {
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/clone", orgName, srcEnvProject, srcEnvName)
	return pc.restCall(ctx, http.MethodPost, path, nil, destEnv, nil)
}

func (pc *client) GetEnvironment(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	version string,
	decrypt bool,
) ([]byte, string, int, error) {
	path, err := pc.resolveEnvironmentPath(orgName, projectName, envName, version)
	if err != nil {
		return nil, "", 0, err
	}
	if decrypt {
		path += "/decrypt"
	}

	var resp *http.Response
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return nil, "", 0, err
	}
	yaml, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", 0, err
	}
	tag := resp.Header.Get(etagHeader)
	revision, err := strconv.Atoi(resp.Header.Get(revisionHeader))
	if err != nil {
		return nil, "", 0, fmt.Errorf("parsing revision number: %w", err)
	}

	return yaml, tag, revision, nil
}

// Deprecated: Use UpdateEnvironmentWithProject instead
func (pc *client) UpdateEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
	yaml []byte,
	tag string,
) ([]EnvironmentDiagnostic, error) {
	return pc.UpdateEnvironmentWithProject(ctx, orgName, DefaultProject, envName, yaml, tag)
}

func (pc *client) UpdateEnvironmentWithProject(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	yaml []byte,
	tag string,
) ([]EnvironmentDiagnostic, error) {
	diags, _, err := pc.UpdateEnvironmentWithRevision(ctx, orgName, projectName, envName, yaml, tag)
	return diags, err
}

func (pc *client) UpdateEnvironmentWithRevision(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	yaml []byte,
	tag string,
) ([]EnvironmentDiagnostic, int, error) {
	header := http.Header{}
	if tag != "" {
		header.Set(etagHeader, tag)
	}

	var errResp EnvironmentErrorResponse
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v", orgName, projectName, envName)
	var resp *http.Response
	err := pc.restCallWithOptions(ctx, http.MethodPatch, path, nil, json.RawMessage(yaml), &resp, httpCallOptions{
		Header:        header,
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest && len(diags.Diagnostics) != 0 {
			return diags.Diagnostics, 0, nil
		}
		return nil, 0, err
	}

	revision, err := strconv.Atoi(resp.Header.Get(revisionHeader))
	if err != nil {
		return nil, 0, fmt.Errorf("parsing revision number: %w", err)
	}

	return nil, revision, nil
}

func (pc *client) DeleteEnvironment(ctx context.Context, orgName, projectName, envName string) error {
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v", orgName, projectName, envName)
	return pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (pc *client) OpenEnvironment(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	version string,
	duration time.Duration,
) (string, []EnvironmentDiagnostic, error) {
	path, err := pc.resolveEnvironmentPath(orgName, projectName, envName, version)
	if err != nil {
		return "", nil, err
	}
	path += "/open"

	queryObj := struct {
		Duration string `url:"duration"`
	}{
		Duration: duration.String(),
	}

	var resp struct {
		ID string `json:"id"`
	}
	var errResp EnvironmentErrorResponse
	err = pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, nil, &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest && len(diags.Diagnostics) != 0 {
			return "", diags.Diagnostics, nil
		}
		return "", nil, err
	}
	return resp.ID, nil, nil
}

func (pc *client) CheckYAMLEnvironment(
	ctx context.Context,
	orgName string,
	yaml []byte,
	opts ...CheckYAMLOption,
) (*esc.Environment, []EnvironmentDiagnostic, error) {

	// NOTE: ideally this method would take a plain old bool as its last parameter: it's not really a public API, so we
	// reserve the right to make breaking changes, including adding parameters.
	//
	// However, there is currently a cyclic dependency between the Pulumi CLI and the ESC CLI that causes breaking changes
	// to any part of the client API to break the ESC CLI build. So we're limited to non-breaking changes, including adding
	// variadic args or adding additional methods.

	path := fmt.Sprintf("/api/esc/environments/%v/yaml/check", orgName)

	queryObj := struct {
		ShowSecrets bool `url:"showSecrets"`
	}{
		ShowSecrets: firstOrDefault(opts).ShowSecrets,
	}

	var resp esc.Environment
	var errResp EnvironmentErrorResponse
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, json.RawMessage(yaml), &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest && len(diags.Diagnostics) != 0 {
			return nil, diags.Diagnostics, nil
		}
		return nil, nil, err
	}
	return &resp, nil, nil
}

func (pc *client) OpenYAMLEnvironment(
	ctx context.Context,
	orgName string,
	yaml []byte,
	duration time.Duration,
) (string, []EnvironmentDiagnostic, error) {
	queryObj := struct {
		Duration string `url:"duration"`
	}{
		Duration: duration.String(),
	}

	var resp struct {
		ID string `json:"id"`
	}
	var errResp EnvironmentErrorResponse
	path := fmt.Sprintf("/api/esc/environments/%v/yaml/open", orgName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, json.RawMessage(yaml), &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest && len(diags.Diagnostics) != 0 {
			return "", diags.Diagnostics, nil
		}
		return "", nil, err
	}
	return resp.ID, nil, nil
}

// Deprecated
func (pc *client) GetOpenEnvironment(ctx context.Context, orgName, envName, openSessionID string) (*esc.Environment, error) {
	return pc.GetOpenEnvironmentWithProject(ctx, orgName, DefaultProject, envName, openSessionID)
}

func (pc *client) GetOpenEnvironmentWithProject(ctx context.Context, orgName, projectName, envName, openSessionID string) (*esc.Environment, error) {
	var resp esc.Environment
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/open/%v", orgName, projectName, envName, openSessionID)
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (pc *client) GetAnonymousOpenEnvironment(ctx context.Context, orgName, openSessionID string) (*esc.Environment, error) {
	var resp esc.Environment
	path := fmt.Sprintf("/api/esc/environments/%s/yaml/open/%s", orgName, openSessionID)
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (pc *client) GetOpenProperty(ctx context.Context, orgName, projectName, envName, openSessionID, property string) (*esc.Value, error) {
	queryObj := struct {
		Property string `url:"property"`
	}{
		Property: property,
	}

	var resp esc.Value
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/open/%v", orgName, projectName, envName, openSessionID)
	err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (pc *client) GetAnonymousOpenProperty(ctx context.Context, orgName, openSessionID, property string) (*esc.Value, error) {
	queryObj := struct {
		Property string `url:"property"`
	}{
		Property: property,
	}

	var resp esc.Value
	path := fmt.Sprintf("/api/esc/environments/%s/yaml/open/%s", orgName, openSessionID)
	err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

type ListEnvironmentTagsOptions struct {
	After string `url:"after"`
	Count *int   `url:"count"`
}

// ListEnvironmentTags lists the tags for the given environment.
func (pc *client) ListEnvironmentTags(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	options ListEnvironmentTagsOptions,
) ([]*EnvironmentTag, string, error) {
	var resp ListEnvironmentTagsResponse
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/tags", orgName, projectName, envName)
	err := pc.restCall(ctx, http.MethodGet, path, options, nil, &resp)
	if err != nil {
		return nil, "", err
	}

	tags := []*EnvironmentTag{}
	for _, t := range resp.Tags {
		tags = append(tags, t)
	}
	return tags, resp.NextToken, nil
}

// CreateEnvironmentTag creates and applies a tag to the given environment.
func (pc *client) CreateEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, key, value string,
) (*EnvironmentTag, error) {
	var resp EnvironmentTag
	req := CreateEnvironmentTagRequest{
		Name:  key,
		Value: value,
	}
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/tags", orgName, projectName, envName)
	err := pc.restCall(ctx, http.MethodPost, path, nil, &req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (pc *client) GetEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, key string,
) (*EnvironmentTag, error) {
	var resp EnvironmentTag
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/tags/%v", orgName, projectName, envName, key)
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateEnvironmentTag updates a specified environment tag with a new key / value.
func (pc *client) UpdateEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, currentKey, currentValue, newKey, newValue string,
) (*EnvironmentTag, error) {
	var resp EnvironmentTag
	req := UpdateEnvironmentTagRequest{
		CurrentTag: TagRequest{
			Value: currentValue,
		},
		NewTag: TagRequest{},
	}
	if newKey != "" {
		req.NewTag.Name = newKey
	}
	if newValue != "" {
		req.NewTag.Value = newValue
	}
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/tags/%v", orgName, projectName, envName, currentKey)
	err := pc.restCall(ctx, http.MethodPatch, path, nil, &req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteEnvironmentTag deletes a specified tag on an environment.
func (pc *client) DeleteEnvironmentTag(ctx context.Context, orgName, projectName, envName, tagName string) error {
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/tags/%v", orgName, projectName, envName, tagName)
	return pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
}

// GetEnvironmentRevision returns a description of the given revision.
func (pc *client) GetEnvironmentRevision(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	revision int,
) (*EnvironmentRevision, error) {
	before, count := revision+1, 1

	opts := ListEnvironmentRevisionsOptions{Before: &before, Count: &count}
	revs, err := pc.ListEnvironmentRevisions(ctx, orgName, projectName, envName, opts)
	if err != nil || len(revs) == 0 {
		return nil, err
	}
	return &revs[0], nil
}

type ListEnvironmentRevisionsOptions struct {
	Before *int `url:"before"`
	Count  *int `url:"count"`
}

func (pc *client) ListEnvironmentRevisions(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	options ListEnvironmentRevisionsOptions,
) ([]EnvironmentRevision, error) {
	var resp []EnvironmentRevision
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions", orgName, projectName, envName)
	err := pc.restCall(ctx, http.MethodGet, path, options, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// RetractEnvironmentRevision retracts a specific revision of an environment.
func (pc *client) RetractEnvironmentRevision(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	version string,
	replacement *int,
	reason string,
) error {
	req := RetractEnvironmentRevisionRequest{
		Replacement: replacement,
		Reason:      reason,
	}
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/%v/retract", orgName, projectName, envName, version)
	return pc.restCall(ctx, http.MethodPost, path, nil, &req, nil)
}

// CreateEnvironmentRevisionTag creates a new revision tag with the given name.
func (pc *client) CreateEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
	revision *int,
) error {
	req := CreateEnvironmentRevisionTagRequest{Name: tagName, Revision: revision}
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/tags", orgName, projectName, envName)
	return pc.restCall(ctx, http.MethodPost, path, nil, &req, nil)
}

// GetEnvironmentRevisionTag returns a description of the given revision tag.
func (pc *client) GetEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
) (*EnvironmentRevisionTag, error) {
	var resp EnvironmentRevisionTag
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/tags/%v", orgName, projectName, envName, tagName)
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateEnvironmentRevisionTag updates the revision tag with the given name.
func (pc *client) UpdateEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
	revision *int,
) error {
	req := UpdateEnvironmentRevisionTagRequest{Revision: revision}
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/tags/%v", orgName, projectName, envName, tagName)
	return pc.restCall(ctx, http.MethodPatch, path, nil, &req, nil)
}

// DeleteEnvironmentRevisionTag deletes the revision tag with the given name.
func (pc *client) DeleteEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
) error {
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/tags/%v", orgName, projectName, envName, tagName)
	return pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
}

type ListEnvironmentRevisionTagsOptions struct {
	After string `url:"after"`
	Count *int   `url:"count"`
}

// ListEnvironmentRevisionTags lists the revision tags for the given environment.
func (pc *client) ListEnvironmentRevisionTags(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	options ListEnvironmentRevisionTagsOptions,
) ([]EnvironmentRevisionTag, error) {
	var resp ListEnvironmentRevisionTagsResponse
	path := fmt.Sprintf("/api/esc/environments/%v/%v/%v/versions/tags", orgName, projectName, envName)
	err := pc.restCall(ctx, http.MethodGet, path, options, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}

// EnvironmentExists checks if the specified environment exists.
func (pc *client) EnvironmentExists(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
) (bool, error) {
	path, err := pc.resolveEnvironmentPath(orgName, projectName, envName, "")
	if err != nil {
		return false, err
	}

	var resp *http.Response
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return false, err
	}

	return true, nil
}

type httpCallOptions struct {
	// RetryPolicy defines the policy for retrying requests by httpClient.Do.
	//
	// By default, only GET requests are retried.
	RetryPolicy retryPolicy

	// GzipCompress compresses the request using gzip before sending it.
	GzipCompress bool

	// Header is any additional headers to add to the request.
	Header http.Header

	// ErrorResponse is an optional response body for errors.
	ErrorResponse any
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *client) restCall(ctx context.Context, method, path string, queryObj, reqObj, respObj interface{}) error {
	return pc.restCallWithOptions(ctx, method, path, queryObj, reqObj, respObj, httpCallOptions{})
}

// restCallWithOptions makes a REST-style request to the Pulumi API using the given method, path, query object, and
// request object. If a response object is provided, the server's response is deserialized into that object.
func (pc *client) restCallWithOptions(
	ctx context.Context,
	method string,
	path string,
	queryObj any,
	reqObj any,
	respObj any,
	opts httpCallOptions,
) error {
	// Compute query string from query object
	querystring := ""
	if queryObj != nil {
		queryValues, err := query.Values(queryObj)
		if err != nil {
			return fmt.Errorf("marshalling query object as JSON: %w", err)
		}
		query := queryValues.Encode()
		if len(query) > 0 {
			querystring = "?" + query
		}
	}

	// Compute request body from request object
	var reqBody []byte
	var err error
	if reqObj != nil {
		// Send verbatim if already marshalled. This is
		// important when sending indented JSON is needed.
		if raw, ok := reqObj.(json.RawMessage); ok {
			reqBody = []byte(raw)
		} else {
			reqBody, err = json.Marshal(reqObj)
			if err != nil {
				return fmt.Errorf("marshalling request object as JSON: %w", err)
			}
		}
	}

	// Make API call
	resp, err := pc.httpCall(ctx, method, path+querystring, reqBody, opts)
	if err != nil {
		return err
	}
	if respPtr, ok := respObj.(**http.Response); ok {
		*respPtr = resp
		return nil
	}

	// Read API response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from API: %w", err)
	}

	if respObj != nil {
		switch respObj := respObj.(type) {
		case *[]byte:
			// Return the raw bytes of the response body.
			*respObj = respBody
		case []byte:
			return fmt.Errorf("Can't unmarshal response body to []byte. Try *[]byte")
		default:
			// Else, unmarshal as JSON.
			if err = json.Unmarshal(respBody, respObj); err != nil {
				return fmt.Errorf("unmarshalling response object: %w", err)
			}
		}
	}

	return nil
}

// httpCall makes an HTTP request to the Pulumi API.
func (pc *client) httpCall(ctx context.Context, method, path string, body []byte, opts httpCallOptions) (*http.Response, error) {
	// Normalize URL components
	cloudAPI := strings.TrimSuffix(pc.apiURL, "/")
	path = cleanPath(path)

	url := cloudAPI + path
	var bodyReader io.Reader
	if opts.GzipCompress {
		// If we're being asked to compress the payload, go ahead and do it here to an intermediate buffer.
		//
		// If this becomes a performance bottleneck, we may want to consider marshaling json directly to this
		// gzip.Writer instead of marshaling to a byte array and compressing it to another buffer.
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		defer contract.IgnoreClose(writer)
		if _, err := writer.Write(body); err != nil {
			return nil, fmt.Errorf("compressing payload: %w", err)
		}

		// gzip.Writer will not actually write anything unless it is flushed,
		//  and it will not actually write the GZip footer unless it is closed. (Close also flushes)
		// Without this, the compressed bytes do not decompress properly e.g. in python.
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("closing compressed payload: %w", err)
		}

		bodyReader = &buf
	} else {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating new HTTP request: %w", err)
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Set headers from the incoming options.
	for k, v := range opts.Header {
		req.Header[k] = v
	}

	// Add a User-Agent header to allow for the backend to make breaking API changes while preserving
	// backwards compatibility.
	req.Header.Set("User-Agent", pc.userAgent)
	// Specify the specific API version we accept.
	req.Header.Set("Accept", "application/vnd.pulumi+8")

	// Apply credentials if provided.
	if pc.apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", pc.apiToken))
	}

	if opts.GzipCompress {
		// If we're sending something that's gzipped, set that header too.
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := doWithRetry(pc.httpClient, req, opts.RetryPolicy)
	if err != nil {
		// Don't wrap *apitype.ErrorResponse.
		if _, ok := err.(*apitype.ErrorResponse); ok {
			return nil, err
		}
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	// Provide a better error if using an authenticated call without having logged in first.
	if resp.StatusCode == 401 && pc.apiToken == "" {
		return nil, errors.New("this command requires logging in; try running `esc login` first")
	}

	// Provide a better error if rate-limit is exceeded(429: Too Many Requests)
	if resp.StatusCode == 429 {
		return nil, errors.New("esc: request rate-limit exceeded")
	}

	// For 4xx and 5xx failures, attempt to provide better diagnostics about what may have gone wrong.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
		// type, and if not just return the raw response text.
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("API call failed (%s), could not read response: %w", resp.Status, err)
		}
		return nil, decodeError(respBody, resp.StatusCode, opts)
	}

	return resp, nil
}

func decodeError(respBody []byte, statusCode int, opts httpCallOptions) error {
	if opts.ErrorResponse != nil {
		if err := json.Unmarshal(respBody, opts.ErrorResponse); err == nil {
			return opts.ErrorResponse.(error)
		}
	}

	var errResp apitype.ErrorResponse
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		errResp.Code = statusCode
		errResp.Message = strings.TrimSpace(string(respBody))
	}
	return &errResp
}

// cleanPath returns the canonical path for p, eliminating . and .. elements.
// Borrowed from gorilla/mux.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}

	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)

	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}

	return np
}

// IsNotFound returns true if the indicated error is a "not found" error.
func IsNotFound(err error) bool {
	resp, ok := err.(*apitype.ErrorResponse)
	return ok && resp.Code == http.StatusNotFound
}

func firstOrDefault[T any](ts []T) (t T) {
	if len(ts) > 0 {
		return ts[0]
	}
	return t
}
