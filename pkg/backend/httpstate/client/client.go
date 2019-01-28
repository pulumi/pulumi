// Copyright 2016-2018, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Client provides a slim wrapper around the Pulumi HTTP/REST API.
type Client struct {
	apiURL   string
	apiToken apiAccessToken
	apiUser  string
	diag     diag.Sink
}

// NewClient creates a new Pulumi API client with the given URL and API token.
func NewClient(apiURL, apiToken string, d diag.Sink) *Client {
	return &Client{
		apiURL:   apiURL,
		apiToken: apiAccessToken(apiToken),
		diag:     d,
	}
}

// apiCall makes a raw HTTP request to the Pulumi API using the given method, path, and request body.
func (pc *Client) apiCall(ctx context.Context, method, path string, body []byte) (string, *http.Response, error) {
	return pulumiAPICall(ctx, pc.diag, pc.apiURL, method, path, body, pc.apiToken, httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCall(ctx context.Context, method, path string, queryObj, reqObj, respObj interface{}) error {
	return pulumiRESTCall(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCallWithOptions(ctx context.Context, method, path string, queryObj, reqObj,
	respObj interface{}, opts httpCallOptions) error {
	return pulumiRESTCall(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, opts)
}

// updateRESTCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. The call is authorized with the indicated update token. If a response object is provided, the server's
// response is deserialized into that object.
func (pc *Client) updateRESTCall(ctx context.Context, method, path string, queryObj, reqObj, respObj interface{},
	token updateAccessToken, httpOptions httpCallOptions) error {

	return pulumiRESTCall(ctx, pc.diag, pc.apiURL, method, path, queryObj, reqObj, respObj, token, httpOptions)
}

// getStackPath returns the API path to for the given stack with the given components joined with path separators
// and appended to the stack root.
func getStackPath(stack StackIdentifier, components ...string) string {
	prefix := fmt.Sprintf("/api/stacks/%s/%s/%s", stack.Owner, stack.Project, stack.Stack)
	return path.Join(append([]string{prefix}, components...)...)
}

// getUpdatePath returns the API path to for the given stack with the given components joined with path separators
// and appended to the update root.
func getUpdatePath(update UpdateIdentifier, components ...string) string {
	components = append([]string{string(apitype.UpdateUpdate), update.UpdateID}, components...)
	return getStackPath(update.StackIdentifier, components...)
}

// GetPulumiAccountName returns the user implied by the API token associated with this client.
func (pc *Client) GetPulumiAccountName(ctx context.Context) (string, error) {
	if pc.apiUser == "" {
		resp := struct {
			GitHubLogin string `json:"githubLogin"`
		}{}
		if err := pc.restCall(ctx, "GET", "/api/user", nil, nil, &resp); err != nil {
			return "", err
		}

		if resp.GitHubLogin == "" {
			return "", errors.New("unexpected response from server")
		}

		pc.apiUser = resp.GitHubLogin
	}

	return pc.apiUser, nil
}

// DownloadPlugin downloads the indicated plugin from the Pulumi API.
func (pc *Client) DownloadPlugin(ctx context.Context, info workspace.PluginInfo, os,
	arch string) (io.ReadCloser, int64, error) {

	endpoint := fmt.Sprintf("/releases/plugins/pulumi-%s-%s-v%s-%s-%s.tar.gz",
		info.Kind, info.Name, info.Version, os, arch)
	_, resp, err := pc.apiCall(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

// GetCLIVersionInfo asks the service for information about versions of the CLI (the newest version as well as the
// oldest version before the CLI should warn about an upgrade).
func (pc *Client) GetCLIVersionInfo(ctx context.Context) (semver.Version, semver.Version, error) {
	var versionInfo apitype.CLIVersionResponse

	if err := pc.restCall(ctx, "GET", "/api/cli/version", nil, nil, &versionInfo); err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	latestSem, err := semver.ParseTolerant(versionInfo.LatestVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	oldestSem, err := semver.ParseTolerant(versionInfo.OldestWithoutWarning)
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	return latestSem, oldestSem, nil
}

// ListStacks lists all stacks the current user has access to, optionally filtered by project.
func (pc *Client) ListStacks(ctx context.Context, projectFilter *string) ([]apitype.StackSummary, error) {

	var resp apitype.ListStacksResponse
	var queryFilter interface{}
	if projectFilter != nil {
		queryFilter = struct {
			ProjectFilter string `url:"project"`
		}{ProjectFilter: *projectFilter}
	}

	if err := pc.restCall(ctx, "GET", "/api/user/stacks", queryFilter, nil, &resp); err != nil {
		return nil, err
	}

	return resp.Stacks, nil
}

// GetLatestConfiguration returns the configuration for the latest deployment of a given stack.
func (pc *Client) GetLatestConfiguration(ctx context.Context, stackID StackIdentifier) (config.Map, error) {
	latest := struct {
		Info apitype.UpdateInfo `json:"info,allowEmpty"`
	}{}

	if err := pc.restCall(ctx, "GET", getStackPath(stackID, "updates", "latest"), nil, nil, &latest); err != nil {
		if restErr, ok := err.(*apitype.ErrorResponse); ok {
			if restErr.Code == http.StatusNotFound {
				return nil, backend.ErrNoPreviousDeployment
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
		if v.Secret {
			cfg[newKey] = config.NewSecureValue(v.String)
		} else {
			cfg[newKey] = config.NewValue(v.String)
		}
	}

	return cfg, nil
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
	ctx context.Context, stackID StackIdentifier, tags map[apitype.StackTagName]string) (apitype.Stack, error) {
	// Validate names and tags.
	if err := backend.ValidateStackProperties(stackID.Stack, tags); err != nil {
		return apitype.Stack{}, errors.Wrap(err, "validating stack properties")
	}

	stack := apitype.Stack{
		StackName:   tokens.QName(stackID.Stack),
		ProjectName: stackID.Project,
		OrgName:     stackID.Owner,
		Tags:        tags,
	}
	createStackReq := apitype.CreateStackRequest{
		StackName: stackID.Stack,
		Tags:      tags,
	}

	var createStackResp apitype.CreateStackResponse

	endpoint := fmt.Sprintf("/api/stacks/%s/%s", stackID.Owner, stackID.Project)
	if err := pc.restCall(
		ctx, "POST", endpoint, nil, &createStackReq, &createStackResp); err != nil {
		return apitype.Stack{}, err
	}

	return stack, nil
}

// DeleteStack deletes the indicated stack. If force is true, the stack is deleted even if it contains resources.
func (pc *Client) DeleteStack(ctx context.Context, stack StackIdentifier, force bool) (bool, error) {
	path := getStackPath(stack)
	if force {
		path += "?force=true"
	}

	// TODO[pulumi/pulumi-service#196] When the service returns a well known response for "this stack still has
	//     resources and `force` was not true", we should sniff for that message and return a true for the boolean.
	return false, pc.restCall(ctx, "DELETE", path, nil, nil, nil)
}

// EncryptValue encrypts a plaintext value in the context of the indicated stack.
func (pc *Client) EncryptValue(ctx context.Context, stack StackIdentifier, plaintext []byte) ([]byte, error) {
	req := apitype.EncryptValueRequest{Plaintext: plaintext}
	var resp apitype.EncryptValueResponse
	if err := pc.restCall(ctx, "POST", getStackPath(stack, "encrypt"), nil, &req, &resp); err != nil {
		return nil, err
	}
	return resp.Ciphertext, nil
}

// DecryptValue decrypts a ciphertext value in the context of the indicated stack.
func (pc *Client) DecryptValue(ctx context.Context, stack StackIdentifier, ciphertext []byte) ([]byte, error) {
	req := apitype.DecryptValueRequest{Ciphertext: ciphertext}
	var resp apitype.DecryptValueResponse
	if err := pc.restCall(ctx, "POST", getStackPath(stack, "decrypt"), nil, &req, &resp); err != nil {
		return nil, err
	}
	return resp.Plaintext, nil
}

// GetStackLogs retrieves the log entries for the indicated stack that match the given query.
func (pc *Client) GetStackLogs(ctx context.Context, stack StackIdentifier,
	logQuery operations.LogQuery) ([]operations.LogEntry, error) {

	var response apitype.LogsResult
	if err := pc.restCall(ctx, "GET", getStackPath(stack, "logs"), logQuery, nil, &response); err != nil {
		return nil, err
	}

	logs := make([]operations.LogEntry, 0, len(response.Logs))
	for _, entry := range response.Logs {
		logs = append(logs, operations.LogEntry(entry))
	}

	return logs, nil
}

// GetStackUpdates returns all updates to the indicated stack.
func (pc *Client) GetStackUpdates(ctx context.Context, stack StackIdentifier) ([]apitype.UpdateInfo, error) {
	var response apitype.GetHistoryResponse
	if err := pc.restCall(ctx, "GET", getStackPath(stack, "updates"), nil, nil, &response); err != nil {
		return nil, err
	}

	return response.Updates, nil
}

// ExportStackDeployment exports the indicated stack's deployment as a raw JSON message.
func (pc *Client) ExportStackDeployment(ctx context.Context,
	stack StackIdentifier) (apitype.UntypedDeployment, error) {

	var resp apitype.ExportStackResponse
	if err := pc.restCall(ctx, "GET", getStackPath(stack, "export"), nil, nil, &resp); err != nil {
		return apitype.UntypedDeployment{}, err
	}

	return apitype.UntypedDeployment(resp), nil
}

// ImportStackDeployment imports a new deployment into the indicated stack.
func (pc *Client) ImportStackDeployment(ctx context.Context, stack StackIdentifier,
	deployment *apitype.UntypedDeployment) (UpdateIdentifier, error) {

	var resp apitype.ImportStackResponse
	if err := pc.restCall(ctx, "POST", getStackPath(stack, "import"), nil, deployment, &resp); err != nil {
		return UpdateIdentifier{}, err
	}

	return UpdateIdentifier{
		StackIdentifier: stack,
		UpdateKind:      apitype.UpdateUpdate,
		UpdateID:        resp.UpdateID,
	}, nil
}

// CreateUpdate creates a new update for the indicated stack with the given kind and assorted options. If the update
// requires that the Pulumi program is uploaded, the provided getContents callback will be invoked to fetch the
// contents of the Pulumi program.
func (pc *Client) CreateUpdate(
	ctx context.Context, kind apitype.UpdateKind, stack StackIdentifier, proj *workspace.Project, cfg config.Map,
	m apitype.UpdateMetadata, opts engine.UpdateOptions, dryRun bool) (UpdateIdentifier, error) {

	// First create the update program request.
	wireConfig := make(map[string]apitype.ConfigValue)
	for k, cv := range cfg {
		v, err := cv.Value(config.NopDecrypter)
		contract.AssertNoError(err)

		wireConfig[k.String()] = apitype.ConfigValue{
			String: v,
			Secret: cv.Secure(),
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
			Analyzers:            opts.Analyzers,
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
	case apitype.UpdateUpdate:
		endpoint = "update"
	case apitype.PreviewUpdate:
		endpoint = "preview"
	case apitype.RefreshUpdate:
		endpoint = "refresh"
	case apitype.DestroyUpdate:
		endpoint = "destroy"
	default:
		contract.Failf("Unknown kind: %s", kind)
	}

	path := getStackPath(stack, endpoint)
	var updateResponse apitype.UpdateProgramResponse
	if err := pc.restCall(ctx, "POST", path, nil, &updateRequest, &updateResponse); err != nil {
		return UpdateIdentifier{}, err
	}

	return UpdateIdentifier{
		StackIdentifier: stack,
		UpdateKind:      kind,
		UpdateID:        updateResponse.UpdateID,
	}, nil
}

// StartUpdate starts the indicated update. It returns the new version of the update's target stack and the token used
// to authenticate operations on the update if any. Replaces the stack's tags with the updated set.
func (pc *Client) StartUpdate(ctx context.Context, update UpdateIdentifier,
	tags map[apitype.StackTagName]string) (int, string, error) {

	// Validate names and tags.
	if err := backend.ValidateStackProperties(update.StackIdentifier.Stack, tags); err != nil {
		return 0, "", errors.Wrap(err, "validating stack properties")
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

// GetUpdateEvents returns all events, taking an optional continuation token from a previous call.
func (pc *Client) GetUpdateEvents(ctx context.Context, update UpdateIdentifier,
	continuationToken *string) (apitype.UpdateResults, error) {

	path := getUpdatePath(update)
	if continuationToken != nil {
		path += fmt.Sprintf("?continuationToken=%s", *continuationToken)
	}

	var results apitype.UpdateResults
	if err := pc.restCall(ctx, "GET", path, nil, nil, &results); err != nil {
		return apitype.UpdateResults{}, err
	}

	return results, nil
}

// RenewUpdateLease renews the indicated update lease for the given duration.
func (pc *Client) RenewUpdateLease(ctx context.Context, update UpdateIdentifier, token string,
	duration time.Duration) (string, error) {

	req := apitype.RenewUpdateLeaseRequest{
		Token:    token,
		Duration: int(duration / time.Second),
	}
	var resp apitype.RenewUpdateLeaseResponse

	// While renewing a lease uses POST, it is safe to send multiple requests (consider that we do this multiple times
	// during a long running update).  Since we would fail our update operation if we can't renew our lease, we'll retry
	// these POST operations.
	if err := pc.restCallWithOptions(ctx, "POST", getUpdatePath(update, "renew_lease"), nil,
		req, &resp, httpCallOptions{RetryAllMethods: true}); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// InvalidateUpdateCheckpoint invalidates the checkpoint for the indicated update.
func (pc *Client) InvalidateUpdateCheckpoint(ctx context.Context, update UpdateIdentifier, token string) error {
	req := apitype.PatchUpdateCheckpointRequest{
		IsInvalid: true,
	}

	// It is safe to retry this PATCH operation, because it is logically idempotent.
	return pc.updateRESTCall(ctx, "PATCH", getUpdatePath(update, "checkpoint"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryAllMethods: true})
}

// PatchUpdateCheckpoint patches the checkpoint for the indicated update with the given contents.
func (pc *Client) PatchUpdateCheckpoint(ctx context.Context, update UpdateIdentifier, deployment *apitype.DeploymentV3,
	token string) error {

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
		updateAccessToken(token), httpCallOptions{RetryAllMethods: true, GzipCompress: true})
}

// CancelUpdate cancels the indicated update.
func (pc *Client) CancelUpdate(ctx context.Context, update UpdateIdentifier) error {

	// It is safe to retry this PATCH operation, because it is logically idempotent.
	return pc.restCallWithOptions(ctx, "POST", getUpdatePath(update, "cancel"), nil, nil, nil,
		httpCallOptions{RetryAllMethods: true})
}

// CompleteUpdate completes the indicated update with the given status.
func (pc *Client) CompleteUpdate(ctx context.Context, update UpdateIdentifier, status apitype.UpdateStatus,
	token string) error {

	req := apitype.CompleteUpdateRequest{
		Status: status,
	}

	// It is safe to retry this PATCH operation, because it is logically idempotent.
	return pc.updateRESTCall(ctx, "POST", getUpdatePath(update, "complete"), nil, req, nil,
		updateAccessToken(token), httpCallOptions{RetryAllMethods: true})
}

// RecordEngineEvent posts an engine event to the Pulumi service.
func (pc *Client) RecordEngineEvent(
	ctx context.Context, update UpdateIdentifier, event apitype.EngineEvent, token string) error {
	callOpts := httpCallOptions{
		GzipCompress:    true,
		RetryAllMethods: true,
	}
	return pc.updateRESTCall(
		ctx, "POST", getUpdatePath(update, "events"),
		nil, event, nil,
		updateAccessToken(token), callOpts)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (pc *Client) UpdateStackTags(
	ctx context.Context, stack StackIdentifier, tags map[apitype.StackTagName]string) error {

	// Validate stack tags.
	if err := backend.ValidateStackTags(tags); err != nil {
		return err
	}

	return pc.restCall(ctx, "PATCH", getStackPath(stack, "tags"), nil, tags, nil)
}
