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

// CreateStackReferenceRequest creates a stack reference to the specified stack.
type CreateStackReferenceRequest struct {
	// OrgName is the organization name the stack being referenced is found in.
	OrgName string `json:"orgName"`
	// ProjectName is the name of the project the stack being referenced is associated with.
	ProjectName string `json:"projectName"`
	// StackName is the name of the stack being referenced.
	StackName string `json:"stackName"`
}

// DeleteStackReferenceRequest deletes the stack reference to the specified stack.
type DeleteStackReferenceRequest struct {
	// OrgName is the organization name the stack being referenced is found in.
	OrgName string `json:"orgName"`
	// ProjectName is the name of the project the stack being referenced is associated with.
	ProjectName string `json:"projectName"`
	// StackName is the name of the stack being referenced.
	StackName string `json:"stackName"`
}
