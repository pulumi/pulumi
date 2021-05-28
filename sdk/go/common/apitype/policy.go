// Copyright 2016-2019, Pulumi Corporation.
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

	// Disabled is an enforcement level that disables the policy from being enforced.
	Disabled EnforcementLevel = "disabled"
)

// IsValid returns true if the EnforcementLevel is a valid value.
func (el EnforcementLevel) IsValid() bool {
	switch el {
	case Advisory, Mandatory, Disabled:
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

// UpdatePolicyGroupRequest modifies a Policy Group.
type UpdatePolicyGroupRequest struct {
	NewName *string `json:"newName,omitempty"`

	AddStack    *PulumiStackReference `json:"addStack,omitempty"`
	RemoveStack *PulumiStackReference `json:"removeStack,omitempty"`

	AddPolicyPack    *PolicyPackMetadata `json:"addPolicyPack,omitempty"`
	RemovePolicyPack *PolicyPackMetadata `json:"removePolicyPack,omitempty"`
}

// PulumiStackReference contains the StackName and ProjectName of the stack.
type PulumiStackReference struct {
	Name           string `json:"name"`
	RoutingProject string `json:"routingProject"`
}

// PolicyPackMetadata is the metadata of a Policy Pack.
type PolicyPackMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     int    `json:"version"`
	VersionTag  string `json:"versionTag"`

	// The configuration that is to be passed to the Policy Pack. This
	// map ties Policies with their configuration.
	Config map[string]*json.RawMessage `json:"config,omitempty"`
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
	Name                  string `json:"name"`
	IsOrgDefault          bool   `json:"isOrgDefault"`
	NumStacks             int    `json:"numStacks"`
	NumEnabledPolicyPacks int    `json:"numEnabledPolicyPacks"`
}

// GetPolicyPackConfigSchemaResponse is the response that includes the JSON
// schemas of Policies within a particular Policy Pack.
type GetPolicyPackConfigSchemaResponse struct {
	// The JSON schema for each Policy's configuration.
	ConfigSchema map[string]PolicyConfigSchema `json:"configSchema,omitempty"`
}
