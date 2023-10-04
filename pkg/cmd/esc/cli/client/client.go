// Copyright 2016-2022, Pulumi Corporation.

package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/pulumi/esc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Client provides a slim wrapper around the Pulumi HTTP/REST API.
type Client interface {
	// Returns true if this client is insecure (i.e. has TLS disabled).
	Insecure() bool

	// URL returns the URL of the API endpoint this client interacts with
	URL() string

	// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
	GetPulumiAccountDetails(ctx context.Context) (string, []string, *workspace.TokenInformation, error)

	ListEnvironments(
		ctx context.Context,
		orgName string,
		continuationToken string,
	) ([]OrgEnvironment, string, error)

	CreateEnvironment(ctx context.Context, orgName, envName string) error

	GetEnvironment(ctx context.Context, orgName, envName string) ([]byte, string, error)

	UpdateEnvironment(
		ctx context.Context,
		orgName string,
		envName string,
		yaml []byte,
		tag string,
	) ([]EnvironmentDiagnostic, error)

	DeleteEnvironment(ctx context.Context, orgName, envName string) error

	OpenEnvironment(
		ctx context.Context,
		orgName string,
		envName string,
		duration time.Duration,
	) (string, []EnvironmentDiagnostic, error)

	CheckYAMLEnvironment(
		ctx context.Context,
		orgName string,
		yaml []byte,
	) (*esc.Environment, []EnvironmentDiagnostic, error)

	OpenYAMLEnvironment(
		ctx context.Context,
		orgName string,
		yaml []byte,
		duration time.Duration,
	) (string, []EnvironmentDiagnostic, error)

	GetOpenEnvironment(ctx context.Context, openEnvID string) (*esc.Environment, error)

	GetOpenProperty(ctx context.Context, openEnvID, property string) (*esc.Value, error)
}

type client struct {
	apiURL     string
	apiToken   string
	apiUser    string
	apiOrgs    []string
	tokenInfo  *workspace.TokenInformation // might be nil if running against old services
	insecure   bool
	restClient restClient
	httpClient *http.Client
}

// newClient creates a new Pulumi API client with the given URL and API token. It is a variable instead of a regular
// function so it can be set to a different implementation at runtime, if necessary.
var newClient = func(userAgent, apiURL, apiToken string, insecure bool) *client {
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

	return &client{
		apiURL:     apiURL,
		apiToken:   apiToken,
		httpClient: httpClient,
		restClient: &defaultRESTClient{
			client: &defaultHTTPClient{
				client: httpClient,
			},
			userAgent: userAgent,
		},
	}
}

// Returns true if this client is insecure (i.e. has TLS disabled).
func (pc *client) Insecure() bool {
	return pc.insecure
}

// New creates a new Pulumi API client with the given URL and API token.
func New(userAgent, apiURL, apiToken string, insecure bool) Client {
	return newClient(userAgent, apiURL, apiToken, insecure)
}

// URL returns the URL of the API endpoint this client interacts with
func (pc *client) URL() string {
	return pc.apiURL
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *client) restCall(ctx context.Context, method, path string, queryObj, reqObj, respObj interface{}) error {
	return pc.restClient.Call(ctx, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken,
		httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *client) restCallWithOptions(ctx context.Context, method, path string, queryObj, reqObj,
	respObj interface{}, opts httpCallOptions,
) error {
	return pc.restClient.Call(ctx, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, opts)
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
	err := pc.restCall(ctx, http.MethodGet, "/api/preview/environments", queryObj, nil, &resp)
	if err != nil {
		return nil, "", err
	}
	return resp.Environments, resp.NextToken, nil
}

func (pc *client) CreateEnvironment(ctx context.Context, orgName, envName string) error {
	path := fmt.Sprintf("/api/preview/environments/%v/%v", orgName, envName)
	return pc.restCall(ctx, http.MethodPost, path, nil, nil, nil)
}

func (pc *client) GetEnvironment(ctx context.Context, orgName, envName string) ([]byte, string, error) {
	path := fmt.Sprintf("/api/preview/environments/%v/%v", orgName, envName)
	var resp *http.Response
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return nil, "", err
	}
	yaml, err := readBody(resp)
	if err != nil {
		return nil, "", err
	}
	tag := resp.Header.Get("ETag")
	return yaml, tag, nil
}

func (pc *client) UpdateEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
	yaml []byte,
	tag string,
) ([]EnvironmentDiagnostic, error) {
	header := http.Header{}
	if tag != "" {
		header.Set("ETag", tag)
	}

	var errResp EnvironmentDiagnosticsResponse
	path := fmt.Sprintf("/api/preview/environments/%v/%v", orgName, envName)
	err := pc.restCallWithOptions(ctx, http.MethodPatch, path, nil, json.RawMessage(yaml), nil, httpCallOptions{
		Header:        header,
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentDiagnosticsResponse
		if errors.As(err, &diags) {
			return diags.Diagnostics, nil
		}
		return nil, err
	}
	return nil, nil
}

func (pc *client) DeleteEnvironment(ctx context.Context, orgName, envName string) error {
	path := fmt.Sprintf("/api/preview/environments/%v/%v", orgName, envName)
	return pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (pc *client) OpenEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
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
	var errResp EnvironmentDiagnosticsResponse
	path := fmt.Sprintf("/api/preview/environments/%v/%v/open", orgName, envName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, nil, &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentDiagnosticsResponse
		if errors.As(err, &diags) {
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
) (*esc.Environment, []EnvironmentDiagnostic, error) {
	var resp esc.Environment
	var errResp EnvironmentDiagnosticsResponse
	path := fmt.Sprintf("/api/preview/environments-yaml/%v/check", orgName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, nil, json.RawMessage(yaml), &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentDiagnosticsResponse
		if errors.As(err, &diags) {
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
	var errResp EnvironmentDiagnosticsResponse
	path := fmt.Sprintf("/api/preview/environments-yaml/%v/open", orgName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, json.RawMessage(yaml), &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentDiagnosticsResponse
		if errors.As(err, &diags) {
			return "", diags.Diagnostics, nil
		}
		return "", nil, err
	}
	return resp.ID, nil, nil
}

func (pc *client) GetOpenEnvironment(ctx context.Context, openEnvID string) (*esc.Environment, error) {
	var resp esc.Environment
	path := fmt.Sprintf("/api/preview/environments-open/%v", openEnvID)
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (pc *client) GetOpenProperty(ctx context.Context, openEnvID, property string) (*esc.Value, error) {
	queryObj := struct {
		Property string `url:"property"`
	}{
		Property: property,
	}

	var resp esc.Value
	path := fmt.Sprintf("/api/preview/openenvironments/%v", openEnvID)
	err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
