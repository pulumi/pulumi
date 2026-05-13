// Copyright 2026, Pulumi Corporation.
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

// EnvironmentReferrer describes a single referrer of an ESC environment.
// Exactly one of Environment, Stack, or InsightsAccount is populated.
type EnvironmentReferrer struct {
	// Environment is set when the referrer is another ESC environment that
	// imports the target environment.
	Environment *EnvironmentImportReferrer `json:"environment,omitempty"`
	// Stack is set when the referrer is a Pulumi stack whose configuration
	// imports the target environment.
	Stack *EnvironmentStackReferrer `json:"stack,omitempty"`
	// InsightsAccount is set when the referrer is a Pulumi Insights account
	// that uses the target environment.
	InsightsAccount *EnvironmentInsightsAccountReferrer `json:"insightsAccount,omitempty"`
}

// EnvironmentImportReferrer identifies another ESC environment that imports
// the target environment.
type EnvironmentImportReferrer struct {
	// Project is the project name of the referring environment.
	Project string `json:"project"`
	// Name is the name of the referring environment.
	Name string `json:"name"`
	// Revision is the revision number of the referring environment.
	Revision int `json:"revision"`
}

// EnvironmentStackReferrer identifies a stack whose configuration imports the
// target environment.
type EnvironmentStackReferrer struct {
	// Project is the project name of the referring stack.
	Project string `json:"project"`
	// Stack is the name of the referring stack.
	Stack string `json:"stack"`
	// Version is the version of the stack update that references the target
	// environment.
	Version int `json:"version"`
}

// EnvironmentInsightsAccountReferrer identifies an Insights account that uses
// the target environment.
type EnvironmentInsightsAccountReferrer struct {
	// AccountName is the name of the Insights account that references the
	// target environment.
	AccountName string `json:"accountName"`
}

// ListEnvironmentReferrersResponse is the response shape of
// `GET /api/esc/environments/{org}/{project}/{env}/referrers`.
//
// Referrers is a map keyed by revision tag or revision number (e.g. "latest"
// or "3") whose value is the slice of referrers that point at that revision.
type ListEnvironmentReferrersResponse struct {
	Referrers         map[string][]EnvironmentReferrer `json:"referrers"`
	ContinuationToken string                           `json:"continuationToken,omitempty"`
}
