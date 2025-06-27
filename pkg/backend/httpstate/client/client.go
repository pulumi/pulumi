// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/blang/semver"
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/util/validation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	// 20s before we give up on a copilot request
	CopilotRequestTimeout = 20 * time.Second
)

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
	apiToken   apiAccessToken
	apiUser    string
	apiOrgs    []string
	tokenInfo  *workspace.TokenInformation // might be nil if running against old services
	diag       diag.Sink
	insecure   bool
	restClient restClient
	httpClient *http.Client

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
		apiURL:     apiURL,
		apiToken:   apiAccessToken(apiToken),
		diag:       d,
		httpClient: httpClient,
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
	pc.httpClient = httpClient
	pc.restClient = &defaultRESTClient{
		client: &defaultHTTPClient{
			client: httpClient,
		},
	}
	return pc
}

// URL returns the URL of the API endpoint this client interacts with
func (pc *Client) URL() string {
	return pc.apiURL
}

// do proxies the http client's Do method. This is a low-level construct and should be used sparingly
func (pc *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return pc.httpClient.Do(req.WithContext(ctx))
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCall(ctx context.Context, method, path string, queryObj, reqObj, respObj interface{}) error {
	return pc.restClient.Call(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken,
		httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCallWithOptions(ctx context.Context, method, path string, queryObj, reqObj,
	respObj interface{}, opts httpCallOptions,
) error {
	return pc.restClient.Call(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, opts)
}

// updateRESTCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. The call is authorized with the indicated update token. If a response object is provided, the server's
// response is deserialized into that object.
func (pc *Client) updateRESTCall(ctx context.Context, method, path string, queryObj, reqObj, respObj interface{},
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
	return fmt.Sprintf("/api/preview/registry/packages/%s/%s/%s/versions", source, publisher, name)
}

func completePackagePublishPath(source, publisher, name, version string) string {
	return fmt.Sprintf("/api/preview/registry/packages/%s/%s/%s/versions/%s/complete", source, publisher, name, version)
}

func publishTemplatePath(source, publisher, name string) string {
	return fmt.Sprintf("/api/preview/registry/templates/%s/%s/%s/versions", source, publisher, name)
}

func completeTemplatePublishPath(source, publisher, name string, version semver.Version) string {
	return fmt.Sprintf("/api/preview/registry/templates/%s/%s/%s/versions/%s/complete", source, publisher, name, version)
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
) (semver.Version, semver.Version, semver.Version, int, error) {
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
		return semver.Version{}, semver.Version{}, semver.Version{}, 0, err
	}

	latestSem, err := semver.ParseTolerant(versionInfo.LatestVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, 0, err
	}

	oldestSem, err := semver.ParseTolerant(versionInfo.OldestWithoutWarning)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, 0, err
	}

	// If there is no dev version, return the latest and oldest
	// versions.  This can happen if the server does not include
	// https://github.com/pulumi/pulumi-service/pull/17429 yet
	if versionInfo.LatestDevVersion == "" {
		return latestSem, oldestSem, semver.Version{}, versionInfo.CacheMS, nil
	}

	devSem, err := semver.ParseTolerant(versionInfo.LatestDevVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, 0, err
	}

	return latestSem, oldestSem, devSem, versionInfo.CacheMS, nil
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

// GetLatestConfiguration returns the configuration for the latest deployment of a given stack.
func (pc *Client) GetLatestConfiguration(ctx context.Context, stackID StackIdentifier) (config.Map, error) {
	latest := getLatestConfigurationResponse{}
	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "updates", "latest"), nil, nil, &latest); err != nil {
		if restErr, ok := err.(*apitype.ErrorResponse); ok {
			if restErr.Code == http.StatusNotFound {
				return nil, ErrNoPreviousDeployment
			}
		}

		return nil, err
	}

	cfg := make(config.Map)
	for k, v := range latest.Info.Config {
		newKey, err := config.ParseKey(k)
		if err != nil {
			return nil, err
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

	return cfg, nil
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

// CreateStack creates a stack with the given cloud and stack name in the scope of the indicated project.
func (pc *Client) CreateStack(
	ctx context.Context,
	stackID StackIdentifier,
	tags map[apitype.StackTagName]string,
	teams []string,
	state *apitype.UntypedDeployment,
	config *apitype.StackConfig,
) (apitype.Stack, error) {
	// Validate names and tags.
	if err := validation.ValidateStackTags(tags); err != nil {
		return apitype.Stack{}, fmt.Errorf("validating stack properties: %w", err)
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

	endpoint := fmt.Sprintf("/api/stacks/%s/%s", stackID.Owner, stackID.Project)
	if err := pc.restCall(
		ctx, "POST", endpoint, nil, &createStackReq, nil); err != nil {
		return apitype.Stack{}, err
	}

	return stack, nil
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

// ExportStackDeployment exports the indicated stack's deployment as a raw JSON message.
// If version is nil, will export the latest version of the stack.
func (pc *Client) ExportStackDeployment(
	ctx context.Context, stack StackIdentifier, version *int,
) (apitype.UntypedDeployment, error) {
	tracingSpan, childCtx := opentracing.StartSpanFromContext(ctx, "ExportStackDeployment")
	defer tracingSpan.Finish()

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
	Messages                    []apitype.Message
	RequiredPolicies            []apitype.RequiredPolicy
	IsCopilotIntegrationEnabled bool
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
			Messages:                    updateResponse.Messages,
			RequiredPolicies:            updateResponse.RequiredPolicies,
			IsCopilotIntegrationEnabled: updateResponse.AISettings.CopilotIsEnabled,
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
) (int, string, error) {
	// Validate names and tags.
	if err := validation.ValidateStackTags(tags); err != nil {
		return 0, "", fmt.Errorf("validating stack properties: %w", err)
	}

	req := apitype.StartUpdateRequest{
		Tags: tags,
	}

	var resp apitype.StartUpdateResponse
	if err := pc.restCall(ctx, "POST", getUpdatePath(update), nil, req, &resp); err != nil {
		return 0, "", err
	}

	return resp.Version, resp.Token, nil
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

// PublishPolicyPack publishes a `PolicyPack` to the Pulumi service. If it successfully publishes
// the Policy Pack, it returns the version of the pack.
func (pc *Client) PublishPolicyPack(ctx context.Context, orgName string,
	analyzerInfo plugin.AnalyzerInfo, dirArchive io.Reader,
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
		}
	}

	req := apitype.CreatePolicyPackRequest{
		Name:        analyzerInfo.Name,
		DisplayName: analyzerInfo.DisplayName,
		VersionTag:  analyzerInfo.Version,
		Policies:    policies,
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

	_, err = pc.httpClient.Do(putReq)
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

// DownloadPolicyPack applies a `PolicyPack` to the Pulumi organization.
func (pc *Client) DownloadPolicyPack(ctx context.Context, url string) (io.ReadCloser, error) {
	getS3Req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to download compressed PolicyPack: %w", err)
	}

	resp, err := pc.httpClient.Do(getS3Req)
	if err != nil {
		return nil, fmt.Errorf("Failed to download compressed PolicyPack: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to download compressed PolicyPack: %s", resp.Status)
	}

	return resp.Body, nil
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
func (pc *Client) PatchUpdateCheckpoint(ctx context.Context, update UpdateIdentifier, deployment *apitype.DeploymentV3,
	token UpdateTokenSource,
) error {
	rawDeployment, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	req := apitype.PatchUpdateCheckpointRequest{
		Version:    3,
		Deployment: rawDeployment,
	}

	// It is safe to retry this PATCH operation, because it is logically idempotent, since we send the entire
	// deployment instead of a set of changes to apply.
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "checkpoint"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryPolicy: retryAllMethods, GzipCompress: true})
}

// PatchUpdateCheckpointVerbatim is a variant of PatchUpdateCheckpoint that preserves JSON indentation of the
// UntypedDeployment transferred over the wire.
func (pc *Client) PatchUpdateCheckpointVerbatim(ctx context.Context, update UpdateIdentifier,
	sequenceNumber int, untypedDeploymentBytes json.RawMessage, token UpdateTokenSource,
) error {
	req := apitype.PatchUpdateVerbatimCheckpointRequest{
		Version:           3,
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
	sequenceNumber int, checkpointHash string, deploymentDelta json.RawMessage, token UpdateTokenSource,
) error {
	req := apitype.PatchUpdateCheckpointDeltaRequest{
		Version:         3,
		CheckpointHash:  checkpointHash,
		SequenceNumber:  sequenceNumber,
		DeploymentDelta: deploymentDelta,
	}

	// It is safe to retry because SequenceNumber serves as an idempotency key.
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "checkpointdelta"), nil, req, nil,
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

// GetUpdateEngineEvents returns the engine events for an update.
func (pc *Client) GetUpdateEngineEvents(ctx context.Context, update UpdateIdentifier,
	continuationToken *string,
) (apitype.GetUpdateEventsResponse, error) {
	path := getUpdatePath(update, "events")
	if continuationToken != nil {
		path += "?continuationToken=" + *continuationToken
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

func (pc *Client) UpdateStackDeploymentSettings(ctx context.Context, stack StackIdentifier,
	deployment apitype.DeploymentSettings,
) error {
	return pc.restCall(ctx, "PUT", getStackPath(stack, "deployments", "settings"), nil, deployment, nil)
}

func (pc *Client) EncryptStackDeploymentSettingsSecret(ctx context.Context,
	stack StackIdentifier, secret string,
) (*apitype.SecretValue, error) {
	request := apitype.SecretValue{Value: secret}
	response := apitype.SecretValue{}
	err := pc.restCall(ctx, "POST", getStackPath(stack, "deployments", "settings", "encrypt"), nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
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
	return path.Join(append([]string{prefix}, components...)...)
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

func (pc *Client) GetDeploymentLogs(ctx context.Context, stack StackIdentifier, id,
	token string,
) (*apitype.DeploymentLogs, error) {
	path := getDeploymentPath(stack, id, "logs?continuationToken="+token)
	var resp apitype.DeploymentLogs
	err := pc.restCall(ctx, http.MethodGet, path, nil, nil, &resp)
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
func (pc *Client) SubmitAIPrompt(ctx context.Context, requestBody interface{}) (*http.Response, error) {
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
	res, err := pc.do(ctx, request)
	return res, err
}

// SummarizeErrorWithCopilot summarizes Pulumi Update output using the Copilot API
func (pc *Client) SummarizeErrorWithCopilot(
	ctx context.Context,
	orgID string,
	content string,
	model string,
	maxSummaryLen int,
) (string, error) {
	request := createSummarizeUpdateRequest(content, orgID, model, maxSummaryLen, maxCopilotSummarizeUpdateContentLength)
	return pc.callCopilot(ctx, request)
}

func (pc *Client) ExplainPreviewWithCopilot(
	ctx context.Context,
	orgID string,
	kind string,
	content string,
) (string, error) {
	request := createExplainPreviewRequest(content, orgID, kind, maxCopilotExplainPreviewContentLength)
	return pc.callCopilot(ctx, request)
}

func (pc *Client) callCopilot(ctx context.Context, requestBody interface{}) (string, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, CopilotRequestTimeout)
	defer cancel()

	url := pc.apiURL + "/api/ai/chat/preview"
	apiToken := string(pc.apiToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Pulumi-Source", "Pulumi CLI")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+apiToken)

	resp, err := pc.do(ctx, req)
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
		putReq, err := http.NewRequest(http.MethodPut, url, reader)
		if err != nil {
			return fmt.Errorf("failed to upload %s: %w", fileType, err)
		}
		for k, v := range resp.RequiredHeaders {
			putReq.Header.Add(k, v)
		}

		uploadResp, err := pc.do(ctx, putReq)
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

	putReq, err := http.NewRequest(http.MethodPut, resp.UploadURLs.Archive, input.Archive)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/gzip")

	uploadResp, err := pc.do(ctx, putReq)
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
	url := fmt.Sprintf("/api/preview/registry/packages/%s/%s/%s/versions/%s", source, publisher, name, v)
	var resp apitype.PackageMetadata
	err := pc.restCall(ctx, "GET", url, nil, nil, &resp)
	return resp, err
}

// GetTemplate is a preview API, and should not be used without an approved EOL plan for
// deprecation.
func (pc *Client) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	v := "latest"
	if version != nil {
		v = version.String()
	}
	url := fmt.Sprintf("/api/preview/registry/templates/%s/%s/%s/versions/%s", source, publisher, name, v)
	var resp apitype.TemplateMetadata
	err := pc.restCall(ctx, "GET", url, nil, nil, &resp)
	return resp, err
}

func (pc *Client) ListPackages(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
	url := "/api/preview/registry/packages?limit=499"
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

// ListTemplates is a preview API, and should not be used without an approved EOL plan for
// deprecation.
func (pc *Client) ListTemplates(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
	url := "/api/preview/registry/templates?limit=499"
	if name != nil {
		url += "&name=" + *name
	}

	var continuationToken *string
	return func(f func(apitype.TemplateMetadata, error) bool) {
		for {
			queryURL := url
			if continuationToken != nil {
				queryURL += "&continuationToken=" + *continuationToken
			}
			var resp apitype.ListTemplatesResponse
			err := pc.restCall(ctx, "GET", queryURL, nil, nil, &resp)
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
