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
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/version"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Client provides a slim wrapper around the Pulumi HTTP/REST API.
type Client interface {
	// Insecure returns true if this client is insecure (i.e. has TLS disabled).
	Insecure() bool

	// URL returns the URL of the API endpoint this client interacts with
	URL() string

	// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
	GetPulumiAccountDetails(ctx context.Context) (string, []string, *workspace.TokenInformation, error)

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

	// CreateEnvironment creats an environment named envName in orgName.
	CreateEnvironment(ctx context.Context, orgName, envName string) error

	// GetEnvironment returns the YAML + ETag for the environment envName in org orgName. If decrypt is
	// true, any { fn::secret: { ciphertext: "..." } } constructs in the definition will be decrypted and
	// replaced with { fn::secret: "plaintext" }.
	//
	// The etag returned by GetEnvironment can be passed to UpdateEnvironment in order to avoid RMW issues
	// when editing envirionments.
	GetEnvironment(ctx context.Context, orgName, envName string, decrypt bool) (yaml []byte, etag string, err error)

	// UpdateEnvironment updates the YAML for the environment envName in org orgName.
	//
	// If the new environment definition contains errors, the update will fail with diagnostics.
	//
	// If etag is not the empty string and the environment's current etag does not match the provided etag
	// (i.e. because a different entity has called UpdateEnvironment), the update will fail with a 409
	// error.
	UpdateEnvironment(
		ctx context.Context,
		orgName string,
		envName string,
		yaml []byte,
		etag string,
	) ([]EnvironmentDiagnostic, error)

	// DeleteEnvironment deletes the environment envName in org orgName.
	DeleteEnvironment(ctx context.Context, orgName, envName string) error

	// OpenEnvironment evaluates the environment envName in org orgName and returns the ID of the opened
	// environment. The opened environment will be available for the indicated duration, after which it
	// will expire.
	//
	// If the environment contains errors, the open will fail with diagnostics.
	OpenEnvironment(
		ctx context.Context,
		orgName string,
		envName string,
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

	// GetOpenEnvironment returns the AST, values, and schema for the open environment with ID openEnvID in
	// environment envName and org orgName.
	GetOpenEnvironment(ctx context.Context, orgName, envName, openEnvID string) (*esc.Environment, error)

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
	GetOpenProperty(ctx context.Context, orgName, envName, openEnvID, property string) (*esc.Value, error)
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

func (pc *client) GetEnvironment(ctx context.Context, orgName, envName string, decrypt bool) ([]byte, string, error) {
	path := fmt.Sprintf("/api/preview/environments/%v/%v", orgName, envName)
	if decrypt {
		path += "/decrypt"
	}

	var resp *http.Response
	if err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return nil, "", err
	}
	yaml, err := io.ReadAll(resp.Body)
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

	var errResp EnvironmentErrorResponse
	path := fmt.Sprintf("/api/preview/environments/%v/%v", orgName, envName)
	err := pc.restCallWithOptions(ctx, http.MethodPatch, path, nil, json.RawMessage(yaml), nil, httpCallOptions{
		Header:        header,
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest {
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
	var errResp EnvironmentErrorResponse
	path := fmt.Sprintf("/api/preview/environments/%v/%v/open", orgName, envName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, nil, &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest {
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
	var errResp EnvironmentErrorResponse
	path := fmt.Sprintf("/api/preview/environments/%v/yaml/check", orgName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, nil, json.RawMessage(yaml), &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest {
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
	path := fmt.Sprintf("/api/preview/environments/%v/yaml/open", orgName)
	err := pc.restCallWithOptions(ctx, http.MethodPost, path, queryObj, json.RawMessage(yaml), &resp, httpCallOptions{
		ErrorResponse: &errResp,
	})
	if err != nil {
		var diags *EnvironmentErrorResponse
		if errors.As(err, &diags) && diags.Code == http.StatusBadRequest {
			return "", diags.Diagnostics, nil
		}
		return "", nil, err
	}
	return resp.ID, nil, nil
}

func (pc *client) GetOpenEnvironment(ctx context.Context, orgName, envName, openSessionID string) (*esc.Environment, error) {
	var resp esc.Environment
	path := fmt.Sprintf("/api/preview/environments/%v/%v/open/%v", orgName, envName, openSessionID)
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (pc *client) GetOpenProperty(ctx context.Context, orgName, envName, openSessionID, property string) (*esc.Value, error) {
	queryObj := struct {
		Property string `url:"property"`
	}{
		Property: property,
	}

	var resp esc.Value
	path := fmt.Sprintf("/api/preview/environments/%v/%v/open/%v", orgName, envName, openSessionID)
	err := pc.restCall(ctx, http.MethodGet, path, queryObj, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
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
