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
	PolicyPack
	Policies []Policy `json:"policies"`
}

// CreatePolicyPackResponse returns a URI to upload the Policy Pack zip file to.
type CreatePolicyPackResponse struct {
	UploadURI string `json:"uploadURI"`
}

// RequiredPolicy is the information regarding a particular Policy that is required
// by an organization.
type RequiredPolicy struct {
	PolicyPack
	PackLocation string `json:"packLocation"`
}

// PolicyPack defines a Policy Pack for an organization.
type PolicyPack struct {
	Name        string `json:"name"`
	Version     int    `json:"version"`
	DisplayName string `json:"displayName"`
}

// Policy defines the metadata for an individual Policy within a Policy Pack.
type Policy struct {
	Name             string           `json:"name"`
	DisplayName      string           `json:"displayName"`
	Description      string           `json:"description"`
	EnforcementLevel EnforcementLevel `json:"enforcementLevel"`
	Message          string           `json:"message"`
}

// EnforcementLevel is an enum to determine the enforcement level for a Policy.
type EnforcementLevel int

const (
	// Warning is an enforcement level where the resource is still created, but a
	// message is displayed to the user for informational / warning purposes.
	Warning EnforcementLevel = 0

	// Mandatory is an enforcement level that prevents a resource from being created.
	Mandatory EnforcementLevel = 100
)
