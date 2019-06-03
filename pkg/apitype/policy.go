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
	Name        string   `json:"name"`
	Version     int      `json:"version"`
	DisplayName string   `json:"displayName"`
	Policies    []Policy `json:"policies"`
}

// CreatePolicyPackResponse is the response from creating a Policy Pack. It returns
// a URI to upload the Policy Pack zip file to.
type CreatePolicyPackResponse struct {
	UploadURI string `json:"uploadURI"`
}

// RequiredPolicy is the information regarding a particular Policy that is required
// by an organization.
type RequiredPolicy struct {
	Name         string `json:"name"`
	Version      int    `json:"version"`
	DisplayName  string `json:"displayName"`
	PackLocation string `json:"packLocation"`
}

// Policy defines the metadata for an individual Policy within a Policy Pack.
type Policy struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`

	// Identifier is uniquely identified a Policy. It is a Policy Pack name and version as
	// well as the Policy name encoded as a string.
	// For example, "aws-security@v5@no-public-s3-buckets"
	Identifier string `json:"identifier"`

	// Description is used to provide more context about the purpose of the policy.
	Description      string           `json:"description"`
	EnforcementLevel EnforcementLevel `json:"enforcementLevel"`

	// Message is the message that will be displayed to end users when they violate
	// this policy.
	Message string `json:"message"`
}

// EnforcementLevel is an enum to determine the enforcement level for a Policy.
type EnforcementLevel int

const (
	// Warning is an enforcement level where the resource is still created, but a
	// message is displayed to the user for informational / warning purposes.
	Warning EnforcementLevel = 1

	// Mandatory is an enforcement level that prevents a resource from being created.
	Mandatory EnforcementLevel = 2
)
