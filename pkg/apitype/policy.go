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

	// The Policies outline the specific Policies in the package, and are derived
	// from the package by the CLI.
	Policies []Policy `json:"policies"`
}

// CreatePolicyPackResponse is the response from creating a Policy Pack. It returns
// a URI to upload the Policy Pack zip file to.
type CreatePolicyPackResponse struct {
	Version   int    `json:"version"`
	UploadURI string `json:"uploadURI"`
}

// RequiredPolicy is the information regarding a particular Policy that is required
// by an organization.
type RequiredPolicy struct {

	// The name (unique and URL-safe) of the required Policy Pack.
	Name string `json:"name"`

	// The version of the required Policy Pack.
	Version int `json:"version"`

	// The pretty name of the required Policy Pack.
	DisplayName string `json:"displayName"`

	// Where the Policy Pack can be downloaded from.
	PackLocation string `json:"packLocation,omitempty"`
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
}

// EnforcementLevel indicates how a policy should be enforced
type EnforcementLevel string

const (
	// Advisory is an enforcement level where the resource is still created, but a
	// message is displayed to the user for informational / warning purposes.
	Advisory EnforcementLevel = "advisory"

	// Mandatory is an enforcement level that prevents a resource from being created.
	Mandatory EnforcementLevel = "mandatory"
)

// GetPolicyPackResponse is the response to get a specific Policy Pack's
// metadata and policies.
type GetPolicyPackResponse struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Version     int      `json:"version"`
	Policies    []Policy `json:"policies"`
}

// ApplyPolicyPackRequest is the request to apply a Policy Pack to an organization.
type ApplyPolicyPackRequest struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// GetStackPolicyPacksResponse is the response to getting the applicable Policy Packs
// for a particular stack. This allows the CLI to download the packs prior to
// starting an update.
type GetStackPolicyPacksResponse struct {
	// RequiredPolicies is a list of required Policy Packs to run during the update.
	RequiredPolicies []RequiredPolicy `json:"requiredPolicies,omitempty"`
}
