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

package apitype

import (
	"encoding/json"
	"time"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
)

// DefaultPolicyGroup is the name of the default Policy Group for organizations.
const DefaultPolicyGroup = "default-policy-group"

// CreatePolicyPackRequest defines the request body for creating a new Policy
// Pack for an organization. The request contains the metadata related to the
// Policy Pack.
type CreatePolicyPackRequest struct {
	// Name is a unique URL-safe identifier (at the org level) for the package.
	// If the name has already been used by the organization, then the request will
	// create a new version of the Policy Pack (incremented by one). This is supplied
	// by the CLI.
	Name string `json:"name"`

	// A pretty name for the Policy Pack that is supplied by the package.
	DisplayName string `json:"displayName"`

	// VersionTag is the semantic version of the Policy Pack. One a version is published, it
	// cannot never be republished. Older clients will not have a version tag.
	VersionTag string `json:"versionTag,omitempty"`

	// The Policies outline the specific Policies in the package, and are derived
	// from the package by the CLI.
	Policies []Policy `json:"policies"`

	// A brief description of the policy pack.
	Description string `json:"description,omitempty"`

	// README text about the policy pack.
	Readme string `json:"readme,omitempty"`

	// The cloud provider/platform this policy pack is associated with, e.g. AWS, Azure, etc.
	Provider string `json:"provider,omitempty"`

	// Tags for this policy pack.
	Tags []string `json:"tags,omitempty"`

	// A URL to the repository where the policy pack is defined.
	Repository string `json:"repository,omitempty"`

	// Runtime identifies the analyzer SDK that produced the pack (e.g. "nodejs",
	// "python", "opa"). Empty when produced by an older SDK that does not report it.
	Runtime string `json:"runtime,omitempty"`

	// Metadata contains optional data about the environment performing the publish operation,
	// e.g. the current source code control commit information.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CreatePolicyPackResponse is the response from creating a Policy Pack. It returns
// a URI to upload the Policy Pack zip file to.
type CreatePolicyPackResponse struct {
	Version   int    `json:"version"`
	UploadURI string `json:"uploadURI"`
	// RequiredHeaders represents headers that the CLI must set in order
	// for the upload to succeed.
	RequiredHeaders map[string]string `json:"requiredHeaders,omitempty"`
}

// RequiredPolicy is the information regarding a particular Policy that is required
// by an organization.
type RequiredPolicy struct {
	// The name (unique and URL-safe) of the required Policy Pack.
	Name string `json:"name"`

	// The version of the required Policy Pack.
	Version int `json:"version"`

	// The version tag of the required Policy Pack.
	VersionTag string `json:"versionTag"`

	// The pretty name of the required Policy Pack.
	DisplayName string `json:"displayName"`

	// Where the Policy Pack can be downloaded from.
	PackLocation string `json:"packLocation,omitempty"`

	// The configuration that is to be passed to the Policy Pack. This is map a of policies
	// mapped to their configuration. Each individual configuration must comply with the
	// JSON schema for each Policy within the Policy Pack.
	Config map[string]*json.RawMessage `json:"config,omitempty"`

	// ESC environment references to resolve for this policy pack.
	Environments []string `json:"environments,omitempty"`

	// The active policy exceptions for this Policy Pack, keyed by exception ID. Each value is a PolicyException.
	Exceptions map[string]json.RawMessage `json:"exceptions,omitempty"`
}

// PolicyException excepts the violations it matches, so that they don't fail an operation.
type PolicyException struct {
	// The justification for the exception.
	Reason string `json:"reason"`
	// The violations to except. A violation is excepted when it matches any entry.
	Match []PolicyExceptionMatch `json:"match"`
}

// PolicyExceptionMatch matches the violations of any of its policies on any of its targets.
type PolicyExceptionMatch struct {
	// The names of policies within the Policy Pack.
	Policies []string `json:"policies"`
	// The URNs of the resources the violations are reported against, compared exactly.
	Targets []string `json:"targets"`
}

// PolicyExceptionInfo is a policy exception stored in Pulumi Cloud.
type PolicyExceptionInfo struct {
	PolicyException

	// The exception's ID, unique within the organization, for example `EXC-42`.
	ID string `json:"id"`
	// The name of the Policy Pack the exception applies to.
	PolicyPack string `json:"policyPack"`
	// The exception's status: `active` or `removed`.
	Status string `json:"status"`
	// The change request that created the exception. Empty when it was created directly.
	ChangeRequestID string `json:"changeRequestId,omitempty"`
	// The user who created the exception. Empty when the user no longer exists.
	CreatedBy UserInfo `json:"createdBy,omitzero"`
	// When the exception was created.
	CreatedAt time.Time `json:"createdAt"`
	// The user who removed the exception. Empty when it's active, or the user no longer exists.
	RemovedBy UserInfo `json:"removedBy,omitzero"`
	// When the exception was removed. Zero when it's active.
	RemovedAt time.Time `json:"removedAt,omitzero"`
}

// CreatePolicyExceptionRequest creates a policy exception in Pulumi Cloud.
type CreatePolicyExceptionRequest struct {
	PolicyException

	// The name of the Policy Pack the exception applies to.
	PolicyPack string `json:"policyPack"`
}

// PolicyExceptionDraft is a policy exception requested in a change request, which creates it once approved.
type PolicyExceptionDraft struct {
	CreatePolicyExceptionRequest

	// The ID of the change request.
	ChangeRequestID string `json:"changeRequestId"`
	// The change request's latest revision, which approvals refer to.
	LatestRevisionNumber int `json:"latestRevisionNumber"`
}

// ListPolicyExceptionsResponse is a page of an organization's policy exceptions.
type ListPolicyExceptionsResponse struct {
	// The policy exceptions, most recently created first.
	Exceptions []PolicyExceptionInfo `json:"exceptions"`
	// The token to fetch the next page with. Empty on the last page.
	ContinuationToken string `json:"continuationToken,omitempty"`
}

// Policy is the Cloud SDK's AppPolicy.
type Policy = cloudapitype.AppPolicy

// PolicyConfigSchema is the Cloud SDK's AppPolicyConfigSchema.
type PolicyConfigSchema = cloudapitype.AppPolicyConfigSchema

// PolicyComplianceFramework is the Cloud SDK's AppPolicyComplianceFramework.
type PolicyComplianceFramework = cloudapitype.AppPolicyComplianceFramework

// JSONSchemaType is the Cloud SDK's AppJSONSchemaType.
type JSONSchemaType = cloudapitype.AppJSONSchemaType

// EnforcementLevel is the Cloud SDK's AppEnforcementLevel.
type EnforcementLevel = cloudapitype.AppEnforcementLevel

// PolicySeverity is the Cloud SDK's AppPolicySeverity.
type PolicySeverity = cloudapitype.AppPolicySeverity

// PolicyGroupMode is the Cloud SDK's PolicyGroupMode.
type PolicyGroupMode = cloudapitype.PolicyGroupMode

// EntityType is the Cloud SDK's AppEntityType.
type EntityType = cloudapitype.AppEntityType

// PolicyGroupEntityType is the Cloud SDK's PolicyGroupEntityType.
type PolicyGroupEntityType = cloudapitype.PolicyGroupEntityType

// GetPolicyGroupResponse is the Cloud SDK's PolicyGroup.
type GetPolicyGroupResponse = cloudapitype.PolicyGroup

// CreatePolicyGroupRequest is the Cloud SDK's NewPolicyGroupRequest.
type CreatePolicyGroupRequest = cloudapitype.NewPolicyGroupRequest

// UpdatePolicyGroupRequest is the Cloud SDK's AppUpdatePolicyGroupRequest.
type UpdatePolicyGroupRequest = cloudapitype.AppUpdatePolicyGroupRequest

// PulumiStackReference is the Cloud SDK's AppPulumiStackReference.
type PulumiStackReference = cloudapitype.AppPulumiStackReference

// InsightsAccountReference is the Cloud SDK's AppInsightsAccountReference.
type InsightsAccountReference = cloudapitype.AppInsightsAccountReference

// PolicyPackMetadata is the Cloud SDK's AppPolicyPackMetadata.
type PolicyPackMetadata = cloudapitype.AppPolicyPackMetadata

// PolicyIssue is the Cloud SDK's PolicyIssue.
type PolicyIssue = cloudapitype.PolicyIssue

// GetPolicyIssueResponse is the Cloud SDK's GetPolicyIssueResponse.
type GetPolicyIssueResponse = cloudapitype.GetPolicyIssueResponse

// ListPolicyIssuesRequest is the Cloud SDK's AngularGridGetRowsRequest.
type ListPolicyIssuesRequest = cloudapitype.AngularGridGetRowsRequest

// ListPolicyIssuesResponse is the Cloud SDK's ListPolicyIssuesResponse.
type ListPolicyIssuesResponse = cloudapitype.ListPolicyIssuesResponse

// PolicyIssueSortModel is the Cloud SDK's AngularGridSortModelItem.
type PolicyIssueSortModel = cloudapitype.AngularGridSortModelItem

const (
	Object = cloudapitype.AppJSONSchemaTypeObject

	Advisory  = cloudapitype.AppEnforcementLevelAdvisory
	Mandatory = cloudapitype.AppEnforcementLevelMandatory
	Remediate = cloudapitype.AppEnforcementLevelRemediate
	Disabled  = cloudapitype.AppEnforcementLevelDisabled

	PolicySeverityUnspecified = cloudapitype.AppPolicySeverityUnspecified
	PolicySeverityLow         = cloudapitype.AppPolicySeverityLow
	PolicySeverityMedium      = cloudapitype.AppPolicySeverityMedium
	PolicySeverityHigh        = cloudapitype.AppPolicySeverityHigh
	PolicySeverityCritical    = cloudapitype.AppPolicySeverityCritical

	PolicyGroupModeAudit        = cloudapitype.PolicyGroupModeAudit
	PolicyGroupModePreventative = cloudapitype.PolicyGroupModePreventative

	Stacks   = cloudapitype.AppEntityTypeStacks
	Accounts = cloudapitype.AppEntityTypeAccounts

	PolicyGroupEntityTypeStacks   = cloudapitype.PolicyGroupEntityTypeStacks
	PolicyGroupEntityTypeAccounts = cloudapitype.PolicyGroupEntityTypeAccounts
)

// GetPolicyPackResponse is the response to get a specific Policy Pack's
// metadata and policies.
type GetPolicyPackResponse struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Version     int      `json:"version"`
	VersionTag  string   `json:"versionTag"`
	Policies    []Policy `json:"policies"`
	Applied     bool     `json:"applied"`
}

// GetStackPolicyPacksResponse is the response to getting the applicable Policy Packs
// for a particular stack. This allows the CLI to download the packs prior to
// starting an update.
type GetStackPolicyPacksResponse struct {
	// RequiredPolicies is a list of required Policy Packs to run during the update.
	RequiredPolicies []RequiredPolicy `json:"requiredPolicies,omitempty"`
}

// ListPolicyPacksResponse is the Cloud SDK's AppListPolicyPacksResponse.
type ListPolicyPacksResponse = cloudapitype.AppListPolicyPacksResponse

// PolicyPackWithVersions is the Cloud SDK's AppPolicyPackWithVersions.
type PolicyPackWithVersions = cloudapitype.AppPolicyPackWithVersions

// ListPolicyGroupsResponse lists a summary of the organization's Policy Groups.
type ListPolicyGroupsResponse struct {
	PolicyGroups []PolicyGroupSummary `json:"policyGroups"`
}

// PolicyGroupSummary details the name, applicable stacks and the applied Policy
// Packs for an organization's Policy Group.
type PolicyGroupSummary struct {
	Name                  string          `json:"name"`
	IsOrgDefault          bool            `json:"isOrgDefault"`
	NumStacks             int             `json:"numStacks"`
	NumAccounts           int             `json:"numAccounts,omitempty"`
	EntityType            EntityType      `json:"entityType"`
	Mode                  PolicyGroupMode `json:"mode"`
	NumEnabledPolicyPacks int             `json:"numEnabledPolicyPacks"`
}

// GetPolicyPackConfigSchemaResponse is the response that includes the JSON
// schemas of Policies within a particular Policy Pack.
type GetPolicyPackConfigSchemaResponse struct {
	// The JSON schema for each Policy's configuration.
	ConfigSchema map[string]PolicyConfigSchema `json:"configSchema,omitempty"`
}

// GetPolicyComplianceResultsRequest is the request body for the compliance
// results endpoint (POST /api/orgs/{orgName}/policy-results/compliance).
type GetPolicyComplianceResultsRequest struct {
	// Entity is how to group results: "stack", "account", or "severity".
	Entity string `json:"entity"`
	// ContinuationToken is the pagination token from a previous response.
	ContinuationToken *string `json:"continuationToken,omitempty"`
	// Size is the number of results per page (max 1000).
	Size *int `json:"size,omitempty"`
}

// GetPolicyComplianceResultsResponse is the response from the compliance
// results endpoint.
type GetPolicyComplianceResultsResponse struct {
	// Columns lists the policy group/pack identifiers or severity levels.
	Columns []string `json:"columns"`
	// Rows contains one entry per entity (stack, account, or policy pack).
	Rows []PolicyComplianceResult `json:"rows"`
	// ContinuationToken is set when more pages are available.
	ContinuationToken *string `json:"continuationToken,omitempty"`
}

// PolicyComplianceResult is a single row in the compliance results table.
type PolicyComplianceResult struct {
	// EntityName identifies the entity (e.g. "project/stack" or account name).
	EntityName string `json:"entityName"`
	// Scores is an array correlating 1:1 with Columns. Values are 0-100
	// (compliance %), -1 (N/A), or -2 (config error).
	Scores []int `json:"scores"`
}
