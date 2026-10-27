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

// PolicyGroupEntityType is the Cloud SDK's PolicyGroupEntityType.
type PolicyGroupEntityType = cloudapitype.PolicyGroupEntityType

// PolicyGroupMemberSource is the Cloud SDK's PolicyGroupMemberSource.
type PolicyGroupMemberSource = cloudapitype.PolicyGroupMemberSource

// PermissionBooleanExpression is the Cloud SDK's PermissionBooleanExpression.
type PermissionBooleanExpression = cloudapitype.PermissionBooleanExpression

// GetPolicyGroupResponse is the Cloud SDK's PolicyGroup.
type GetPolicyGroupResponse = cloudapitype.PolicyGroup

// CreatePolicyGroupRequest is the Cloud SDK's NewPolicyGroupRequest.
type CreatePolicyGroupRequest = cloudapitype.NewPolicyGroupRequest

// UpdatePolicyGroupRequest is the Cloud SDK's AppUpdatePolicyGroupRequest.
type UpdatePolicyGroupRequest = cloudapitype.AppUpdatePolicyGroupRequest

// PolicyGroupUpdate is the Cloud SDK's UpdatePolicyGroupRequest.
type PolicyGroupUpdate = cloudapitype.UpdatePolicyGroupRequest

// PulumiStackReference is the Cloud SDK's AppPulumiStackReference.
type PulumiStackReference = cloudapitype.AppPulumiStackReference

// InsightsAccountReference is the Cloud SDK's AppInsightsAccountReference.
type InsightsAccountReference = cloudapitype.AppInsightsAccountReference

// PolicyPackMetadata is the Cloud SDK's AppPolicyPackMetadata.
type PolicyPackMetadata = cloudapitype.AppPolicyPackMetadata

// PolicyGroupStack is the Cloud SDK's PolicyGroupStack.
type PolicyGroupStack = cloudapitype.PolicyGroupStack

// PolicyGroupAccount is the Cloud SDK's PolicyGroupAccount.
type PolicyGroupAccount = cloudapitype.PolicyGroupAccount

// ListPolicyGroupStacksResponse is the Cloud SDK's ListPolicyGroupStacksResponse.
type ListPolicyGroupStacksResponse = cloudapitype.ListPolicyGroupStacksResponse

// ListPolicyGroupAccountsResponse is the Cloud SDK's ListPolicyGroupAccountsResponse.
type ListPolicyGroupAccountsResponse = cloudapitype.ListPolicyGroupAccountsResponse

// RegistryPolicyPack is the Cloud SDK's RegistryPolicyPack.
type RegistryPolicyPack = cloudapitype.RegistryPolicyPack

// RegistryPolicyPackAccessLevel is the Cloud SDK's RegistryPolicyPackAccessLevel.
type RegistryPolicyPackAccessLevel = cloudapitype.RegistryPolicyPackAccessLevel

// PolicyIssue is the Cloud SDK's PolicyIssue.
type PolicyIssue = cloudapitype.PolicyIssue

// PolicyIssueStatus is the Cloud SDK's PolicyIssueStatus.
type PolicyIssueStatus = cloudapitype.PolicyIssueStatus

// PolicyIssuePriority is the Cloud SDK's PolicyIssuePriority.
type PolicyIssuePriority = cloudapitype.PolicyIssuePriority

// PolicyIssueKind is the Cloud SDK's PolicyIssueKind.
type PolicyIssueKind = cloudapitype.PolicyIssueKind

// IssueEntityType is the Cloud SDK's IssueEntityType.
type IssueEntityType = cloudapitype.IssueEntityType

// GetPolicyIssueResponse is the Cloud SDK's GetPolicyIssueResponse.
type GetPolicyIssueResponse = cloudapitype.GetPolicyIssueResponse

// UpdatePolicyIssueRequest is the Cloud SDK's UpdatePolicyIssueRequest.
type UpdatePolicyIssueRequest = cloudapitype.UpdatePolicyIssueRequest

// ListPolicyIssuesRequest is the Cloud SDK's AngularGridGetRowsRequest.
type ListPolicyIssuesRequest = cloudapitype.AngularGridGetRowsRequest

// ListPolicyIssuesResponse is the Cloud SDK's ListPolicyIssuesResponse.
type ListPolicyIssuesResponse = cloudapitype.ListPolicyIssuesResponse

// PolicyIssueSortModel is the Cloud SDK's AngularGridSortModelItem.
type PolicyIssueSortModel = cloudapitype.AngularGridSortModelItem

// AngularGridAdvancedFilterModel is the Cloud SDK's AngularGridAdvancedFilterModel.
type AngularGridAdvancedFilterModel = cloudapitype.AngularGridAdvancedFilterModel

// AngularGridFilterModel is the Cloud SDK's AngularGridFilterModel.
type AngularGridFilterModel = cloudapitype.AngularGridFilterModel

// AngularGridColumn is the Cloud SDK's AngularGridColumn.
type AngularGridColumn = cloudapitype.AngularGridColumn

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

	PolicyGroupEntityTypeStacks   = cloudapitype.PolicyGroupEntityTypeStacks
	PolicyGroupEntityTypeAccounts = cloudapitype.PolicyGroupEntityTypeAccounts

	PolicyGroupMemberSourceExplicit = cloudapitype.PolicyGroupMemberSourceExplicit
	PolicyGroupMemberSourceRule     = cloudapitype.PolicyGroupMemberSourceRule

	RegistryPolicyPackAccessLevelFull     = cloudapitype.RegistryPolicyPackAccessLevelFull
	RegistryPolicyPackAccessLevelViewOnly = cloudapitype.RegistryPolicyPackAccessLevelViewOnly
	RegistryPolicyPackAccessLevelDeny     = cloudapitype.RegistryPolicyPackAccessLevelDeny

	PolicyIssueStatusOpen       = cloudapitype.PolicyIssueStatusOpen
	PolicyIssueStatusInProgress = cloudapitype.PolicyIssueStatusInProgress
	PolicyIssueStatusByDesign   = cloudapitype.PolicyIssueStatusByDesign
	PolicyIssueStatusFixed      = cloudapitype.PolicyIssueStatusFixed
	PolicyIssueStatusIgnored    = cloudapitype.PolicyIssueStatusIgnored

	PolicyIssuePriorityP0 = cloudapitype.PolicyIssuePriorityP0
	PolicyIssuePriorityP1 = cloudapitype.PolicyIssuePriorityP1
	PolicyIssuePriorityP2 = cloudapitype.PolicyIssuePriorityP2
	PolicyIssuePriorityP3 = cloudapitype.PolicyIssuePriorityP3
	PolicyIssuePriorityP4 = cloudapitype.PolicyIssuePriorityP4

	PolicyIssueKindAudit        = cloudapitype.PolicyIssueKindAudit
	PolicyIssueKindPreventative = cloudapitype.PolicyIssueKindPreventative

	IssueEntityTypeStack           = cloudapitype.IssueEntityTypeStack
	IssueEntityTypeInsightsAccount = cloudapitype.IssueEntityTypeInsightsAccount
)

// EntityType indicates the type of entity a policy group applies to
type EntityType string

const (
	// Stacks indicates the policy group applies to stacks
	Stacks EntityType = "stacks"

	// Accounts indicates the policy group applies to accounts
	Accounts EntityType = "accounts"
)

// IsValid returns true if the EntityType is a valid value.
func (et EntityType) IsValid() bool {
	switch et {
	case Stacks, Accounts:
		return true
	}
	return false
}

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

// ListPolicyPacksResponse is the response to list an organization's
// Policy Packs.
type ListPolicyPacksResponse struct {
	PolicyPacks []PolicyPackWithVersions `json:"policyPacks"`
}

// PolicyPackWithVersions details the specifics of a Policy Pack and all its available versions.
type PolicyPackWithVersions struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Versions    []int    `json:"versions"`
	VersionTags []string `json:"versionTags"`
}

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
