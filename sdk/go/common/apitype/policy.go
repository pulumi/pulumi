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

import "encoding/json"

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

	// Runtime of the required Policy Pack (e.g. "nodejs", "oci"). Empty for
	// packs published before runtimes were recorded.
	Runtime string `json:"runtime,omitempty"`

	// ImageRef is the digest-pinned OCI image reference for the required
	// Policy Pack (e.g. "ghcr.io/acme/pack@sha256:…"). Set only for packs
	// published with runtime "oci"; such packs have no PackLocation.
	ImageRef string `json:"imageRef,omitempty"`

	// The configuration that is to be passed to the Policy Pack. This is map a of policies
	// mapped to their configuration. Each individual configuration must comply with the
	// JSON schema for each Policy within the Policy Pack.
	Config map[string]*json.RawMessage `json:"config,omitempty"`

	// ESC environment references to resolve for this policy pack.
	Environments []string `json:"environments,omitempty"`
}

// Policy defines the metadata for an individual Policy within a Policy Pack.
type Policy struct {
	// Unique URL-safe name for the policy.  This is unique to a specific version
	// of a Policy Pack.
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`

	// Description is used to provide more context about the purpose of the policy.
	Description      string           `json:"description"`
	EnforcementLevel EnforcementLevel `json:"enforcementLevel"`

	// Message is the message that will be displayed to end users when they violate
	// this policy.
	Message string `json:"message"`

	// The JSON schema for the Policy's configuration.
	ConfigSchema *PolicyConfigSchema `json:"configSchema,omitempty"`

	// The severity of the policy.
	Severity PolicySeverity `json:"severity,omitempty"`

	// The compliance framework that this policy belongs to.
	Framework *PolicyComplianceFramework `json:"framework,omitempty"`

	// Tags associated with the policy.
	Tags []string `json:"tags,omitempty"`

	// A description of the steps to take to remediate a policy violation.
	RemediationSteps string `json:"remediationSteps,omitempty"`

	// A URL to more information about the policy.
	URL string `json:"url,omitempty"`
}

// PolicyConfigSchema defines the JSON schema of a particular Policy's
// configuration.
type PolicyConfigSchema struct {
	// Config property name to JSON Schema map.
	Properties map[string]*json.RawMessage `json:"properties,omitempty"`
	// Required config properties.
	Required []string `json:"required,omitempty"`

	// Type defines the data type allowed for the schema.
	Type JSONSchemaType `json:"type"`
}

// JSONSchemaType in an enum of allowed data types for a schema.
type JSONSchemaType string

const (
	// Object is a dictionary.
	Object JSONSchemaType = "object"
)

// EnforcementLevel indicates how a policy should be enforced
type EnforcementLevel string

const (
	// Advisory is an enforcement level where the resource is still created, but a
	// message is displayed to the user for informational / warning purposes.
	Advisory EnforcementLevel = "advisory"

	// Mandatory is an enforcement level that prevents a resource from being created.
	Mandatory EnforcementLevel = "mandatory"

	// Remediate is an enforcement level that fixes policy issues instead of issuing diagnostics.
	Remediate EnforcementLevel = "remediate"

	// Disabled is an enforcement level that disables the policy from being enforced.
	Disabled EnforcementLevel = "disabled"
)

// Indicates the severity of a policy.
type PolicySeverity string

const (
	PolicySeverityUnspecified PolicySeverity = ""
	PolicySeverityLow         PolicySeverity = "low"
	PolicySeverityMedium      PolicySeverity = "medium"
	PolicySeverityHigh        PolicySeverity = "high"
	PolicySeverityCritical    PolicySeverity = "critical"
)

// IsValid returns true if the EnforcementLevel is a valid value.
func (el EnforcementLevel) IsValid() bool {
	switch el {
	case Advisory, Mandatory, Remediate, Disabled:
		return true
	}
	return false
}

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

// PolicyGroupMode indicates the enforcement mode of a policy group
type PolicyGroupMode string

const (
	// PolicyGroupModePreventative enforces policies during pulumi up/preview operations,
	// potentially blocking resource changes when mandatory policies fail
	PolicyGroupModePreventative PolicyGroupMode = "preventative"

	// PolicyGroupModeAudit monitors resource compliance without blocking operations,
	// reporting policy violations for continuous compliance monitoring
	PolicyGroupModeAudit PolicyGroupMode = "audit"
)

// IsValid returns true if the PolicyGroupMode is a valid value.
func (m PolicyGroupMode) IsValid() bool {
	switch m {
	case PolicyGroupModePreventative, PolicyGroupModeAudit:
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

// CreatePolicyGroupRequest defines the request body for creating a new Policy
// Group in an organization.
type CreatePolicyGroupRequest struct {
	// Name is the name of the new Policy Group.
	Name string `json:"name"`
	// EntityType is the type of entities this policy group applies to:
	// "stacks" or "accounts".
	EntityType string `json:"entityType"`
	// Mode is the enforcement mode: "audit" or "preventative".
	// If empty, defaults to "audit" for accounts and "preventative" for stacks.
	Mode string `json:"mode,omitempty"`
	// AgentPoolID is the optional agent pool to use for policy evaluation.
	AgentPoolID string `json:"agentPoolId,omitempty"`
}

// UpdatePolicyGroupRequest modifies a Policy Group. Callers may set:
//
//   - NewName to rename the group.
//   - A singular AddX/RemoveX field for a single per-PATCH membership change.
//   - A list field (Stacks, PolicyPacks, InsightsAccounts) to replace the
//     corresponding list outright; the values sent become the new full list.
//     Use the pointer-to-slice indirection to distinguish "leave list
//     unchanged" (nil pointer) from "set list to empty" (non-nil empty slice).
//
// Multiple of these may be combined in a single request to batch changes.
type UpdatePolicyGroupRequest struct {
	NewName *string `json:"newName,omitempty"`

	AddStack    *PulumiStackReference `json:"addStack,omitempty"`
	RemoveStack *PulumiStackReference `json:"removeStack,omitempty"`

	AddPolicyPack    *PolicyPackMetadata `json:"addPolicyPack,omitempty"`
	RemovePolicyPack *PolicyPackMetadata `json:"removePolicyPack,omitempty"`

	AddInsightsAccount    *InsightsAccountReference `json:"addInsightsAccount,omitempty"`
	RemoveInsightsAccount *InsightsAccountReference `json:"removeInsightsAccount,omitempty"`

	// Stacks, when non-nil, replaces the full list of stacks in the group.
	Stacks *[]PulumiStackReference `json:"stacks,omitempty"`
	// PolicyPacks, when non-nil, replaces the full list of Policy Packs
	// applied to the group.
	PolicyPacks *[]PolicyPackMetadata `json:"policyPacks,omitempty"`
	// InsightsAccounts, when non-nil, replaces the full list of Insights
	// accounts in the group.
	InsightsAccounts *[]string `json:"insightsAccounts,omitempty"`
}

// InsightsAccountReference identifies an Insights account for policy group
// membership. The server requires at least the Name field.
type InsightsAccountReference struct {
	Name string `json:"name"`
}

// PulumiStackReference contains the StackName and ProjectName of the stack.
type PulumiStackReference struct {
	Name           string `json:"name"`
	RoutingProject string `json:"routingProject,omitempty"`
}

// PolicyPackMetadata is the metadata of a Policy Pack.
type PolicyPackMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Version     int    `json:"version,omitempty"`
	VersionTag  string `json:"versionTag,omitempty"`

	// The configuration that is to be passed to the Policy Pack. This
	// map ties Policies with their configuration.
	Config map[string]*json.RawMessage `json:"config,omitempty"`

	// ESC environment references to resolve for this policy pack.
	Environments []string `json:"environments,omitempty"`
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

// GetPolicyGroupResponse is the response to get a specific Policy Group's
// metadata, applied Policy Packs, and member stacks or accounts.
type GetPolicyGroupResponse struct {
	// Name is the name of the Policy Group.
	Name string `json:"name"`

	// IsOrgDefault is true if this is either the default stacks or default
	// accounts Policy Group for the organization.
	IsOrgDefault bool `json:"isOrgDefault"`

	// EntityType is the type of entities this Policy Group applies to
	// (stacks or accounts).
	EntityType EntityType `json:"entityType"`

	// Mode is the enforcement mode for the Policy Group (audit or preventative).
	Mode PolicyGroupMode `json:"mode"`

	// Stacks lists the stacks that are members of this Policy Group.
	Stacks []PulumiStackReference `json:"stacks"`

	// AppliedPolicyPacks lists the Policy Packs that are applied to this
	// Policy Group.
	AppliedPolicyPacks []PolicyPackMetadata `json:"appliedPolicyPacks"`

	// Accounts lists the Insights account names that are members of this
	// Policy Group.
	Accounts []string `json:"accounts"`

	// AgentPoolID is the agent pool ID used for audit policy evaluation.
	// Defaults to the Pulumi hosted pool if not specified.
	AgentPoolID string `json:"agentPoolId,omitempty"`
}

// GetPolicyPackConfigSchemaResponse is the response that includes the JSON
// schemas of Policies within a particular Policy Pack.
type GetPolicyPackConfigSchemaResponse struct {
	// The JSON schema for each Policy's configuration.
	ConfigSchema map[string]PolicyConfigSchema `json:"configSchema,omitempty"`
}

// PolicyComplianceFramework represents a compliance framework that a policy belongs to.
type PolicyComplianceFramework struct {
	// The compliance framework name.
	Name string `json:"name,omitempty"`
	// The compliance framework version.
	Version string `json:"version,omitempty"`
	// The compliance framework reference.
	Reference string `json:"reference,omitempty"`
	// The compliance framework specification.
	Specification string `json:"specification,omitempty"`
}

// ListPolicyIssuesRequest is the request body for the ListPolicyIssues endpoint
// (POST /api/orgs/{orgName}/policyresults/issues). The server expects an
// AngularGrid-style request with startRow/endRow pagination.
type ListPolicyIssuesRequest struct {
	StartRow  int                    `json:"startRow"`
	EndRow    int                    `json:"endRow"`
	SortModel []PolicyIssueSortModel `json:"sortModel"`
}

// PolicyIssueSortModel describes a sort column for the policy issues endpoint.
type PolicyIssueSortModel struct {
	ColID string `json:"colId"`
	Sort  string `json:"sort"` // "asc" or "desc"
}

// PolicyIssue is a single policy violation detected by a Policy Pack during a
// stack update or a continuous-compliance scan. Field names match the server's
// JSON response.
type PolicyIssue struct {
	ID               string         `json:"id"`
	EntityType       string         `json:"entityType,omitempty"`
	EntityProject    string         `json:"entityProject,omitempty"`
	EntityID         string         `json:"entityId,omitempty"`
	PolicyPack       string         `json:"policyPack"`
	PolicyPackTag    string         `json:"policyPackTag,omitempty"`
	PolicyName       string         `json:"policyName"`
	Level            string         `json:"level"`
	Severity         PolicySeverity `json:"severity,omitempty"`
	ResourceURN      string         `json:"resourceURN,omitempty"`
	ResourceProvider string         `json:"resourceProvider,omitempty"`
	ResourceType     string         `json:"resourceType,omitempty"`
	ResourceName     string         `json:"resourceName,omitempty"`
	Message          string         `json:"message,omitempty"`
	ObservedAt       string         `json:"observedAt,omitempty"`
	LastModified     string         `json:"lastModified,omitempty"`
	Status           string         `json:"status,omitempty"`
	Kind             string         `json:"kind,omitempty"`
	Priority         string         `json:"priority,omitempty"`
}

// GetPolicyIssueResponse is the response wrapper for the GetPolicyIssue
// endpoint (GET /api/orgs/{orgName}/policyresults/issues/{issueId}).
type GetPolicyIssueResponse struct {
	PolicyIssue PolicyIssue `json:"policyIssue"`
}

// ListPolicyIssuesResponse is the response body for the ListPolicyIssues
// endpoint.
type ListPolicyIssuesResponse struct {
	// Issues is the page of policy issues.
	Issues []PolicyIssue `json:"policyIssues"`
	// Total is the total number of issues across all pages.
	Total int64 `json:"rowCount"`
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
