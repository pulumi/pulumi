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
	"regexp"
	"strconv"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/util/validation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

// URL returns the URL of the API endpoint this client interacts with
func (pc *Client) URL() string {
	return pc.apiURL
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

// ListStacksFilter describes optional filters when listing stacks.
type ListStacksFilter struct {
	Project      *string
	Organization *string
	TagName      *string
	TagValue     *string
}

// ListStacks lists all stacks the current user has access to, optionally filtered by project.
func (pc *Client) ListStacks(
	ctx context.Context, filter ListStacksFilter) ([]apitype.StackSummary, error) {
	queryFilter := struct {
		Project      *string `url:"project,omitempty"`
		Organization *string `url:"organization,omitempty"`
		TagName      *string `url:"tagName,omitempty"`
		TagValue     *string `url:"tagValue,omitempty"`
	}{
		Project:      filter.Project,
		Organization: filter.Organization,
		TagName:      filter.TagName,
		TagValue:     filter.TagValue,
	}

	var resp apitype.ListStacksResponse
	if err := pc.restCall(ctx, "GET", "/api/user/stacks", queryFilter, nil, &resp); err != nil {
		return nil, err
	}

	return resp.Stacks, nil
}

var (
	// ErrNoPreviousDeployment is returned when there isn't a previous deployment.
	ErrNoPreviousDeployment = errors.New("no previous deployment")
)

// GetLatestConfiguration returns the configuration for the latest deployment of a given stack.
func (pc *Client) GetLatestConfiguration(ctx context.Context, stackID StackIdentifier) (config.Map, error) {
	latest := struct {
		Info apitype.UpdateInfo `json:"info,omitempty"`
	}{}

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
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusNotFound {
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
	ctx context.Context, stackID StackIdentifier, tags map[apitype.StackTagName]string) (apitype.Stack, error) {
	// Validate names and tags.
	if err := validation.ValidateStackProperties(stackID.Stack, tags); err != nil {
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

// GetStackUpdates returns all updates to the indicated stack.
func (pc *Client) GetStackUpdates(
	ctx context.Context,
	stack StackIdentifier,
	pageSize int,
	page int) ([]apitype.UpdateInfo, error) {
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
	ctx context.Context, stack StackIdentifier, version *int) (apitype.UntypedDeployment, error) {

	path := getStackPath(stack, "export")

	// Tack on a specific version as desired.
	if version != nil {
		path += fmt.Sprintf("/%d", *version)
	}

	var resp apitype.ExportStackResponse
	if err := pc.restCall(ctx, "GET", path, nil, nil, &resp); err != nil {
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
	ctx context.Context, kind apitype.UpdateKind, stack StackIdentifier, proj *workspace.Project,
	cfg config.Map, m apitype.UpdateMetadata, opts engine.UpdateOptions,
	dryRun bool) (UpdateIdentifier, []apitype.RequiredPolicy, error) {

	// First create the update program request.
	wireConfig := make(map[string]apitype.ConfigValue)
	for k, cv := range cfg {
		v, err := cv.Value(config.NopDecrypter)
		contract.AssertNoError(err)

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
	default:
		contract.Failf("Unknown kind: %s", kind)
	}

	path := getStackPath(stack, endpoint)
	var updateResponse apitype.UpdateProgramResponse
	if err := pc.restCall(ctx, "POST", path, nil, &updateRequest, &updateResponse); err != nil {
		return UpdateIdentifier{}, []apitype.RequiredPolicy{}, err
	}

	return UpdateIdentifier{
		StackIdentifier: stack,
		UpdateKind:      kind,
		UpdateID:        updateResponse.UpdateID,
	}, updateResponse.RequiredPolicies, nil
}

// RenameStack renames the provided stack to have the new identifier.
func (pc *Client) RenameStack(ctx context.Context, currentID, newID StackIdentifier) error {
	req := apitype.StackRenameRequest{
		NewName:    newID.Stack,
		NewProject: newID.Project,
	}
	var resp apitype.ImportStackResponse

	return pc.restCall(ctx, "POST", getStackPath(currentID, "rename"), nil, &req, &resp)
}

// StartUpdate starts the indicated update. It returns the new version of the update's target stack and the token used
// to authenticate operations on the update if any. Replaces the stack's tags with the updated set.
func (pc *Client) StartUpdate(ctx context.Context, update UpdateIdentifier,
	tags map[apitype.StackTagName]string) (int, string, error) {

	// Validate names and tags.
	if err := validation.ValidateStackProperties(update.StackIdentifier.Stack, tags); err != nil {
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

// ListPolicyGroups lists all `PolicyGroups` the organization has in the Pulumi service.
func (pc *Client) ListPolicyGroups(ctx context.Context, orgName string) (apitype.ListPolicyGroupsResponse, error) {
	var resp apitype.ListPolicyGroupsResponse
	err := pc.restCall(ctx, "GET", listPolicyGroupsPath(orgName), nil, nil, &resp)
	if err != nil {
		return resp, errors.Wrapf(err, "List Policy Groups failed")
	}
	return resp, nil
}

// ListPolicyPacks lists all `PolicyPack` the organization has in the Pulumi service.
func (pc *Client) ListPolicyPacks(ctx context.Context, orgName string) (apitype.ListPolicyPacksResponse, error) {
	var resp apitype.ListPolicyPacksResponse
	err := pc.restCall(ctx, "GET", listPolicyPacksPath(orgName), nil, nil, &resp)
	if err != nil {
		return resp, errors.Wrapf(err, "List Policy Packs failed")
	}
	return resp, nil
}

// PublishPolicyPack publishes a `PolicyPack` to the Pulumi service. If it successfully publishes
// the Policy Pack, it returns the version of the pack.
func (pc *Client) PublishPolicyPack(ctx context.Context, orgName string,
	analyzerInfo plugin.AnalyzerInfo, dirArchive io.Reader) (string, error) {

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
		versionMsg = fmt.Sprintf(" - version %s", analyzerInfo.Version)
	}
	fmt.Printf("Publishing %q%s to %q\n", analyzerInfo.Name, versionMsg, orgName)

	var resp apitype.CreatePolicyPackResponse
	err := pc.restCall(ctx, "POST", publishPolicyPackPath(orgName), nil, req, &resp)
	if err != nil {
		return "", errors.Wrapf(err, "Publish policy pack failed")
	}

	//
	// Step 2: Upload the compressed PolicyPack directory to the pre-signed object storage service URL.
	// The PolicyPack is now published.
	//

	putReq, err := http.NewRequest(http.MethodPut, resp.UploadURI, dirArchive)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to upload compressed PolicyPack")
	}

	for k, v := range resp.RequiredHeaders {
		putReq.Header.Add(k, v)
	}

	_, err = http.DefaultClient.Do(putReq)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to upload compressed PolicyPack")
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
		return "", errors.Wrapf(err, "Request to signal completion of the publish operation failed")
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
	policyPackName, versionTag string, policyPackConfig map[string]*json.RawMessage) error {

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
		return errors.Wrapf(err, "Enable policy pack failed")
	}
	return nil
}

// GetPolicyPackSchema gets Policy Pack config schema.
func (pc *Client) GetPolicyPackSchema(ctx context.Context, orgName,
	policyPackName, versionTag string) (*apitype.GetPolicyPackConfigSchemaResponse, error) {
	var resp apitype.GetPolicyPackConfigSchemaResponse
	err := pc.restCall(ctx, http.MethodGet,
		getPolicyPackConfigSchemaPath(orgName, policyPackName, versionTag), nil, nil, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "Retrieving policy pack config schema failed")
	}
	return &resp, nil
}

// DisablePolicyPack disables a `PolicyPack` to the Pulumi organization. If policyGroup is not empty,
// it will disable the PolicyPack on the default PolicyGroup.
func (pc *Client) DisablePolicyPack(ctx context.Context, orgName string, policyGroup string,
	policyPackName, versionTag string) error {

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
		return errors.Wrapf(err, "Request to disable policy pack failed")
	}
	return nil
}

// RemovePolicyPack removes all versions of a `PolicyPack` from the Pulumi organization.
func (pc *Client) RemovePolicyPack(ctx context.Context, orgName string, policyPackName string) error {
	path := deletePolicyPackPath(orgName, policyPackName)
	err := pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "Request to remove policy pack failed")
	}
	return nil
}

// RemovePolicyPackByVersion removes a specific version of a `PolicyPack` from
// the Pulumi organization.
func (pc *Client) RemovePolicyPackByVersion(ctx context.Context, orgName string,
	policyPackName string, versionTag string) error {

	path := deletePolicyPackVersionPath(orgName, policyPackName, versionTag)
	err := pc.restCall(ctx, http.MethodDelete, path, nil, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "Request to remove policy pack failed")
	}
	return nil
}

// DownloadPolicyPack applies a `PolicyPack` to the Pulumi organization.
func (pc *Client) DownloadPolicyPack(ctx context.Context, url string) (io.ReadCloser, error) {
	getS3Req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to download compressed PolicyPack")
	}

	resp, err := http.DefaultClient.Do(getS3Req)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to download compressed PolicyPack")
	}

	return resp.Body, nil
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
		Duration: int(duration / time.Second),
	}
	var resp apitype.RenewUpdateLeaseResponse

	// While renewing a lease uses POST, it is safe to send multiple requests (consider that we do this multiple times
	// during a long running update).  Since we would fail our update operation if we can't renew our lease, we'll retry
	// these POST operations.
	if err := pc.updateRESTCall(ctx, "POST", getUpdatePath(update, "renew_lease"), nil, req, &resp,
		updateAccessToken(token), httpCallOptions{RetryAllMethods: true}); err != nil {
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

// RecordEngineEvents posts a batch of engine events to the Pulumi service.
func (pc *Client) RecordEngineEvents(
	ctx context.Context, update UpdateIdentifier, batch apitype.EngineEventBatch, token string) error {
	callOpts := httpCallOptions{
		GzipCompress:    true,
		RetryAllMethods: true,
	}
	return pc.updateRESTCall(
		ctx, "POST", getUpdatePath(update, "events/batch"),
		nil, batch, nil,
		updateAccessToken(token), callOpts)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (pc *Client) UpdateStackTags(
	ctx context.Context, stack StackIdentifier, tags map[apitype.StackTagName]string) error {

	// Validate stack tags.
	if err := validation.ValidateStackTags(tags); err != nil {
		return err
	}

	return pc.restCall(ctx, "PATCH", getStackPath(stack, "tags"), nil, tags, nil)
}
