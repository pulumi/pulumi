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

// ESCEnvironment is a single Pulumi ESC environment as returned by the ESC
// `ListEnvironments` endpoint (`GET /api/esc/environments/{orgName}`). The
// shape mirrors the OrgEnvironment schema used by the `pulumi esc ls` CLI.
type ESCEnvironment struct {
	// Organization is the org that owns the environment.
	Organization string `json:"organization,omitempty"`
	// Project is the ESC project the environment lives under.
	Project string `json:"project,omitempty"`
	// Name is the human-readable name of the environment within the project.
	Name string `json:"name,omitempty"`
}

// ListESCEnvironmentsResponse is the envelope returned by the ESC
// ListEnvironments endpoint. NextToken is empty on the last page.
type ListESCEnvironmentsResponse struct {
	Environments []ESCEnvironment `json:"environments,omitempty"`
	NextToken    string           `json:"nextToken,omitempty"`
}
