// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
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
}

// NewClient creates a new Pulumi API client with the given URL and API token.
func NewClient(apiURL, apiToken string) *Client {
	return &Client{
		apiURL:   apiURL,
		apiToken: apiAccessToken(apiToken),
	}
}

// apiCall makes a raw HTTP request to the Pulumi API using the given method, path, and request body.
func (pc *Client) apiCall(method, path string, body []byte) (string, *http.Response, error) {
	return pulumiAPICall(pc.apiURL, method, path, body, pc.apiToken, httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCall(method, path string, queryObj, reqObj, respObj interface{}) error {
	return pulumiRESTCall(pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, httpCallOptions{})
}

// restCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. If a response object is provided, the server's response is deserialized into that object.
func (pc *Client) restCallWithOptions(method, path string, queryObj, reqObj,
	respObj interface{}, opts httpCallOptions) error {
	return pulumiRESTCall(pc.apiURL, method, path, queryObj, reqObj, respObj, pc.apiToken, opts)
}

// updateRESTCall makes a REST-style request to the Pulumi API using the given method, path, query object, and request
// object. The call is authorized with the indicated update token. If a response object is provided, the server's
// response is deserialized into that object.
func (pc *Client) updateRESTCall(method, path string, queryObj, reqObj, respObj interface{},
	token updateAccessToken) error {

	return pulumiRESTCall(pc.apiURL, method, path, queryObj, reqObj, respObj, token, httpCallOptions{})
}

// getProjectPath returns the API path to for the given project with the given components joined with path separators
// and appended to the project root.
func getProjectPath(project ProjectIdentifier, components ...string) string {
	contract.Assertf(project.Repository != "", "need repository in ProjectIdentifier")

	projectRoot := fmt.Sprintf("/api/orgs/%s/programs/%s/%s", project.Owner, project.Repository, project.Project)
	return path.Join(append([]string{projectRoot}, components...)...)
}

// getStackPath returns the API path to for the given stack with the given components joined with path separators
// and appended to the stack root.
func getStackPath(stack StackIdentifier, components ...string) string {

	// When stack.Repository is not empty, we are on the old pulumi init based identity plan, and we hit different REST
	// endpoints.
	//
	// TODO(ellismg)[pulumi/pulumi#1241] Clean this up once we remove pulumi init
	if stack.Repository == "" {
		return path.Join(append([]string{fmt.Sprintf("/api/stacks/%s/%s", stack.Owner, stack.Stack)}, components...)...)
	}

	components = append([]string{"stacks", stack.Stack}, components...)
	return getProjectPath(stack.ProjectIdentifier, components...)
}

// getUpdatePath returns the API path to for the given stack with the given components joined with path separators
// and appended to the update root.
func getUpdatePath(update UpdateIdentifier, components ...string) string {
	components = append([]string{string(update.UpdateKind), update.UpdateID}, components...)
	return getStackPath(update.StackIdentifier, components...)
}

// GetPulumiAccountName returns the user implied by the API token associated with this client.
func (pc *Client) GetPulumiAccountName() (string, error) {
	if pc.apiUser == "" {
		resp := struct {
			GitHubLogin string `json:"githubLogin"`
		}{}
		if err := pc.restCall("GET", "/api/user", nil, nil, &resp); err != nil {
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
func (pc *Client) DownloadPlugin(info workspace.PluginInfo, os, arch string) (io.ReadCloser, int64, error) {
	endpoint := fmt.Sprintf("/releases/plugins/pulumi-%s-%s-v%s-%s-%s.tar.gz",
		info.Kind, info.Name, info.Version, os, arch)
	_, resp, err := pc.apiCall("GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

// ListTemplates lists all templates of which the Pulumi API knows.
func (pc *Client) ListTemplates() ([]workspace.Template, error) {
	// Query all templates.
	var templates []workspace.Template
	if err := pc.restCall("GET", "/releases/templates", nil, nil, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// DownloadTemplate downloads the indicated template from the Pulumi API.
func (pc *Client) DownloadTemplate(name string) (io.ReadCloser, int64, error) {
	// Make the GET request to download the template.
	endpoint := fmt.Sprintf("/releases/templates/%s.tar.gz", name)
	_, resp, err := pc.apiCall("GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

// ListStacks lists all stacks for the indicated project.
func (pc *Client) ListStacks(project ProjectIdentifier, projectFilter *tokens.PackageName) ([]apitype.Stack, error) {
	// Query all stacks for the project on Pulumi.
	var stacks []apitype.Stack

	if project.Repository != "" {
		if err := pc.restCall("GET", getProjectPath(project, "stacks"), nil, nil, &stacks); err != nil {
			return nil, err
		}
	} else {
		var queryFilter interface{}
		if projectFilter != nil {
			queryFilter = struct {
				ProjectFilter string `url:"project"`
			}{ProjectFilter: project.Project}
		}

		if err := pc.restCall("GET", "/api/user/stacks", queryFilter, nil, &stacks); err != nil {
			return nil, err
		}
	}

	return stacks, nil
}

// GetStack retrieves the stack with the given name.
func (pc *Client) GetStack(stackID StackIdentifier) (apitype.Stack, error) {
	var stack apitype.Stack
	if err := pc.restCall("GET", getStackPath(stackID), nil, nil, &stack); err != nil {
		return apitype.Stack{}, err
	}
	return stack, nil
}

// CreateStack creates a stack with the given cloud and stack name in the scope of the indicated project.
func (pc *Client) CreateStack(
	project ProjectIdentifier, cloudName string, stackName string,
	tags map[apitype.StackTagName]string) (apitype.Stack, error) {
	// Validate names and tags.
	if err := backend.ValidateStackProperties(stackName, tags); err != nil {
		return apitype.Stack{}, errors.Wrap(err, "validating stack properties")
	}

	stack := apitype.Stack{
		CloudName:   cloudName,
		StackName:   tokens.QName(stackName),
		OrgName:     project.Owner,
		RepoName:    project.Repository,
		ProjectName: project.Project,
		Tags:        tags,
	}
	createStackReq := apitype.CreateStackRequest{
		CloudName: cloudName,
		StackName: stackName,
		Tags:      tags,
	}

	var createStackResp apitype.CreateStackResponseByName
	if project.Repository != "" {
		if err := pc.restCall(
			"POST", getProjectPath(project, "stacks"), nil, &createStackReq, &createStackResp); err != nil {
			return apitype.Stack{}, err
		}
	} else {
		if err := pc.restCall(
			"POST", fmt.Sprintf("/api/stacks/%s", project.Owner), nil, &createStackReq, &createStackResp); err != nil {
			return apitype.Stack{}, err
		}
	}

	stack.CloudName = createStackResp.CloudName
	return stack, nil
}

// DeleteStack deletes the indicated stack. If force is true, the stack is deleted even if it contains resources.
func (pc *Client) DeleteStack(stack StackIdentifier, force bool) (bool, error) {
	path := getStackPath(stack)
	if force {
		path += "?force=true"
	}

	// TODO[pulumi/pulumi-service#196] When the service returns a well known response for "this stack still has
	//     resources and `force` was not true", we should sniff for that message and return a true for the boolean.
	return false, pc.restCall("DELETE", path, nil, nil, nil)
}

// EncryptValue encrypts a plaintext value in the context of the indicated stack.
func (pc *Client) EncryptValue(stack StackIdentifier, plaintext []byte) ([]byte, error) {
	req := apitype.EncryptValueRequest{Plaintext: plaintext}
	var resp apitype.EncryptValueResponse
	if err := pc.restCall("POST", getStackPath(stack, "encrypt"), nil, &req, &resp); err != nil {
		return nil, err
	}
	return resp.Ciphertext, nil
}

// DecryptValue decrypts a ciphertext value in the context of the indicated stack.
func (pc *Client) DecryptValue(stack StackIdentifier, ciphertext []byte) ([]byte, error) {
	req := apitype.DecryptValueRequest{Ciphertext: ciphertext}
	var resp apitype.DecryptValueResponse
	if err := pc.restCall("POST", getStackPath(stack, "decrypt"), nil, &req, &resp); err != nil {
		return nil, err
	}
	return resp.Plaintext, nil
}

// GetStackLogs retrieves the log entries for the indicated stack that match the given query.
func (pc *Client) GetStackLogs(stack StackIdentifier, logQuery operations.LogQuery) ([]operations.LogEntry, error) {
	var response apitype.LogsResult
	if err := pc.restCall("GET", getStackPath(stack, "logs"), logQuery, nil, &response); err != nil {
		return nil, err
	}

	logs := make([]operations.LogEntry, 0, len(response.Logs))
	for _, entry := range response.Logs {
		logs = append(logs, operations.LogEntry(entry))
	}

	return logs, nil
}

// GetStackUpdates returns all updates to the indicated stack.
func (pc *Client) GetStackUpdates(stack StackIdentifier) ([]apitype.UpdateInfo, error) {
	var response apitype.GetHistoryResponse
	if err := pc.restCall("GET", getStackPath(stack, "updates"), nil, nil, &response); err != nil {
		return nil, err
	}

	return response.Updates, nil
}

// ExportStackDeployment exports the indicated stack's deployment as a raw JSON message.
func (pc *Client) ExportStackDeployment(stack StackIdentifier) (apitype.UntypedDeployment, error) {
	var resp apitype.ExportStackResponse
	if err := pc.restCall("GET", getStackPath(stack, "export"), nil, nil, &resp); err != nil {
		return apitype.UntypedDeployment{}, err
	}

	return apitype.UntypedDeployment(resp), nil
}

// ImportStackDeployment imports a new deployment into the indicated stack.
func (pc *Client) ImportStackDeployment(stack StackIdentifier, deployment json.RawMessage) (UpdateIdentifier, error) {
	req := apitype.ImportStackRequest{Deployment: deployment}
	var resp apitype.ImportStackResponse
	if err := pc.restCall("POST", getStackPath(stack, "import"), nil, &req, &resp); err != nil {
		return UpdateIdentifier{}, err
	}

	return UpdateIdentifier{
		StackIdentifier: stack,
		UpdateKind:      UpdateKindUpdate,
		UpdateID:        resp.UpdateID,
	}, nil
}

// CreateUpdate creates a new update for the indicated stack with the given kind and assorted options. If the update
// requires that the Pulumi program is uploaded, the provided getContents callback will be invoked to fetch the
// contents of the Pulumi program.
func (pc *Client) CreateUpdate(
	kind UpdateKind, stack StackIdentifier, pkg *workspace.Project, cfg config.Map,
	main string, m apitype.UpdateMetadata, opts engine.UpdateOptions, dryRun bool,
	getContents func() (io.ReadCloser, int64, error)) (UpdateIdentifier, error) {

	// First create the update program request.
	wireConfig := make(map[string]apitype.ConfigValue)
	for k, cv := range cfg {
		v, err := cv.Value(config.NopDecrypter)
		contract.AssertNoError(err)

		wireConfig[k.Namespace()+":config:"+k.Name()] = apitype.ConfigValue{
			String: v,
			Secret: cv.Secure(),
		}
	}

	description := ""
	if pkg.Description != nil {
		description = *pkg.Description
	}

	updateRequest := apitype.UpdateProgramRequest{
		Name:        string(pkg.Name),
		Runtime:     pkg.Runtime,
		Main:        main,
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
	case UpdateKindUpdate:
		if dryRun {
			endpoint = "preview"
		} else {
			endpoint = "update"
		}
	case UpdateKindRefresh:
		// Issue #1081: this will not work for PPC-managed stacks: instead, it will run an update. We rely on the
		// caller to do the right thing here.
		endpoint, kind = "update", UpdateKindUpdate
	case UpdateKindDestroy:
		endpoint = "destroy"
	default:
		contract.Failf("Unknown kind: %s", kind)
	}

	path := getStackPath(stack, endpoint)
	var updateResponse apitype.UpdateProgramResponse
	if err := pc.restCall("POST", path, nil, &updateRequest, &updateResponse); err != nil {
		return UpdateIdentifier{}, err
	}

	// Now upload the program if necessary.
	if kind != UpdateKindDestroy && updateResponse.UploadURL != "" {
		uploadURL, err := url.Parse(updateResponse.UploadURL)
		if err != nil {
			return UpdateIdentifier{}, errors.Wrap(err, "parsing upload URL")
		}

		contents, size, err := getContents()
		if err != nil {
			return UpdateIdentifier{}, err
		}

		resp, err := http.DefaultClient.Do(&http.Request{
			Method:        "PUT",
			URL:           uploadURL,
			ContentLength: size,
			Body:          contents,
		})
		if err != nil {
			return UpdateIdentifier{}, err
		}
		if resp.StatusCode != http.StatusOK {
			return UpdateIdentifier{}, errors.Errorf("upload failed: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
	}

	return UpdateIdentifier{
		StackIdentifier: stack,
		UpdateKind:      kind,
		UpdateID:        updateResponse.UpdateID,
	}, nil
}

// StartUpdate starts the indicated update. It returns the new version of the update's target stack and the token used
// to authenticate operations on the update if any. Replaces the stack's tags with the updated set.
func (pc *Client) StartUpdate(update UpdateIdentifier, tags map[apitype.StackTagName]string) (int, string, error) {
	// Validate names and tags.
	if err := backend.ValidateStackProperties(update.StackIdentifier.Stack, tags); err != nil {
		return 0, "", errors.Wrap(err, "validating stack properties")
	}

	req := apitype.StartUpdateRequest{
		Tags: tags,
	}

	var resp apitype.StartUpdateResponse
	if err := pc.restCall("POST", getUpdatePath(update), req, nil, &resp); err != nil {
		return 0, "", err
	}

	return resp.Version, resp.Token, nil
}

// GetUpdateEvents returns all events, taking an optional continuation token from a previous call.
func (pc *Client) GetUpdateEvents(update UpdateIdentifier, continuationToken *string) (apitype.UpdateResults, error) {
	path := getUpdatePath(update)
	if continuationToken != nil {
		path += fmt.Sprintf("?continuationToken=%s", *continuationToken)
	}

	var results apitype.UpdateResults
	if err := pc.restCall("GET", path, nil, nil, &results); err != nil {
		return apitype.UpdateResults{}, err
	}

	return results, nil
}

// RenewUpdateLease renews the indicated update lease for the given duration.
func (pc *Client) RenewUpdateLease(update UpdateIdentifier, token string, duration time.Duration) (string, error) {
	req := apitype.RenewUpdateLeaseRequest{
		Token:    token,
		Duration: int(duration / time.Second),
	}
	var resp apitype.RenewUpdateLeaseResponse
	if err := pc.restCallWithOptions("POST", getUpdatePath(update, "renew_lease"), nil,
		req, &resp, httpCallOptions{RetryAllMethods: true}); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// InvalidateUpdateCheckpoint invalidates the checkpoint for the indicated update.
func (pc *Client) InvalidateUpdateCheckpoint(update UpdateIdentifier, token string) error {
	req := apitype.PatchUpdateCheckpointRequest{
		IsInvalid: true,
	}
	return pc.updateRESTCall("PATCH", getUpdatePath(update, "checkpoint"), nil, req, nil, updateAccessToken(token))
}

// PatchUpdateCheckpoint patches the checkpoint for the indicated update with the given contents.
func (pc *Client) PatchUpdateCheckpoint(update UpdateIdentifier, deployment *apitype.DeploymentV1, token string) error {
	rawDeployment, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	req := apitype.PatchUpdateCheckpointRequest{
		Version:    1,
		Deployment: rawDeployment,
	}
	return pc.updateRESTCall("PATCH", getUpdatePath(update, "checkpoint"), nil, req, nil, updateAccessToken(token))
}

// CancelUpdate cancels the indicated update.
func (pc *Client) CancelUpdate(update UpdateIdentifier) error {
	return pc.restCall("POST", getUpdatePath(update, "cancel"), nil, nil, nil)
}

// CompleteUpdate completes the indicated update with the given status.
func (pc *Client) CompleteUpdate(update UpdateIdentifier, status apitype.UpdateStatus, token string) error {
	req := apitype.CompleteUpdateRequest{
		Status: status,
	}
	return pc.updateRESTCall("POST", getUpdatePath(update, "complete"), nil, req, nil, updateAccessToken(token))
}

// AppendUpdateLogEntry appends the given entry to the indicated update's logs.
func (pc *Client) AppendUpdateLogEntry(update UpdateIdentifier, kind string, fields map[string]interface{},
	token string) error {

	req := apitype.AppendUpdateLogEntryRequest{
		Kind:   kind,
		Fields: fields,
	}
	return pc.updateRESTCall("POST", getUpdatePath(update, "log"), nil, req, nil, updateAccessToken(token))
}
