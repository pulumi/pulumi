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

import "time"

// ListDeploymentResponseV2 is the response from the Pulumi Cloud
// ListStackDeploymentsHandlerV2 endpoint: a paginated list of deployments
// for a single stack plus the total count for the stack.
type ListDeploymentResponseV2 struct {
	// Deployments is the page of deployment snapshots.
	Deployments []ListDeploymentSnapshot `json:"deployments"`
	// ItemsPerPage is the page size the server actually applied.
	ItemsPerPage int64 `json:"itemsPerPage"`
	// Total is the total number of deployments for the stack.
	Total int64 `json:"total"`
}

// ListDeploymentSnapshot is a single deployment entry as returned by the
// ListStackDeploymentsHandlerV2 endpoint. The shape mirrors the OpenAPI
// schema of the same name (ListDeploymentResponse + per-snapshot extensions
// flattened together for ergonomics; the wire format is a single object).
type ListDeploymentSnapshot struct {
	// ID uniquely identifies this deployment.
	ID string `json:"id"`
	// Created is when the deployment was created. The wire format is a string
	// (no `date-time` constraint in the OpenAPI spec), so we preserve whatever
	// the server returned verbatim.
	Created string `json:"created"`
	// Modified is when the corresponding WorkflowRun was last modified. Same
	// caveat as Created.
	Modified string `json:"modified"`
	// Status is the deployment status: one of `not-started`, `accepted`,
	// `running`, `failed`, `succeeded`, `skipped`.
	Status string `json:"status"`
	// Version is the ordinal ID for the stack at the time of this deployment.
	Version int64 `json:"version"`
	// RequestedBy describes the user who created the deployment.
	RequestedBy UserInfo `json:"requestedBy"`

	// ProjectName is the name of the project the stack belongs to.
	ProjectName string `json:"projectName,omitempty"`
	// StackName is the name of the stack.
	StackName string `json:"stackName,omitempty"`
	// Paused is true when deployments are paused for the program.
	Paused bool `json:"paused,omitempty"`
	// PulumiOperation is the Pulumi operation that was performed.
	PulumiOperation PulumiOperation `json:"pulumiOperation"`
	// Updates is the list of stack updates produced by this deployment.
	Updates []DeploymentNestedUpdate `json:"updates"`
	// Jobs is the list of jobs run as part of this deployment, with their
	// step-level progress.
	Jobs []DeploymentJob `json:"jobs"`
	// Initiator records the initiation source of the deployment (e.g. `cli`,
	// `webhook`). Empty when the server did not report one.
	Initiator string `json:"initiator,omitempty"`
	// AgentPool is the self-hosted agent pool that ran the deployment, if any.
	AgentPool *ListDeploymentSnapshotAgentPool `json:"agentPool,omitempty"`
}

// UserInfo is the public-facing subset of a Pulumi Cloud user. Email is only
// populated by admin-only APIs and is otherwise absent.
type UserInfo struct {
	Name        string `json:"name"`
	GitHubLogin string `json:"githubLogin"`
	AvatarURL   string `json:"avatarUrl"`
	Email       string `json:"email,omitempty"`
}

// DeploymentJob is a single job within a deployment, with its status, timing,
// and step-level progress.
type DeploymentJob struct {
	// Status is the job status: one of `not-started`, `accepted`, `running`,
	// `failed`, `succeeded`, `skipped`.
	Status string `json:"status"`
	// Started is when the job started; zero when the job has not started yet.
	Started time.Time `json:"started"`
	// LastUpdated is when the job was last updated; zero when never updated.
	LastUpdated time.Time `json:"lastUpdated"`
	// Steps is the list of steps in the job.
	Steps []DeploymentStepRun `json:"steps"`
}

// DeploymentStepRun is a single step within a DeploymentJob.
type DeploymentStepRun struct {
	// Name identifies the step within its job.
	Name string `json:"name"`
	// Status is the step status: one of `not-started`, `running`, `failed`,
	// `succeeded`.
	Status string `json:"status"`
	// Started is when the step started; zero when the step has not started yet.
	Started time.Time `json:"started"`
	// LastUpdated is when the step was last updated; zero when never updated.
	LastUpdated time.Time `json:"lastUpdated"`
}

// DeploymentNestedUpdate is a stack update produced by a deployment. The
// `kind` and `result` fields are kept as raw strings rather than typed
// enums because the server enumerates preview-variant kinds (Pupdate,
// Prefresh, ...) that aren't represented in `apitype.UpdateKind`.
type DeploymentNestedUpdate struct {
	ID          string            `json:"id"`
	UpdateID    string            `json:"updateID"`
	Version     int64             `json:"version"`
	StartTime   int64             `json:"startTime"`
	EndTime     int64             `json:"endTime"`
	Result      string            `json:"result"`
	Kind        string            `json:"kind"`
	Message     string            `json:"message"`
	Environment map[string]string `json:"environment"`
}

// ListDeploymentSnapshotAgentPool is the agent pool that ran a deployment.
type ListDeploymentSnapshotAgentPool struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetDeploymentResponse is the response from the Pulumi Cloud
// `GetDeployment` endpoint
// (GET /api/stacks/{org}/{project}/{stack}/deployments/{deploymentId}).
//
// The shape mirrors ListDeploymentSnapshot — every field returned by the
// list endpoint is also returned by the get endpoint — with the addition
// of `inheritSettings`, which records whether the deployment inherited
// settings from the stack at creation time.
type GetDeploymentResponse struct {
	// ID uniquely identifies this deployment.
	ID string `json:"id"`
	// Created is when the deployment was created. The wire format is a
	// string (no `date-time` constraint in the OpenAPI spec).
	Created string `json:"created"`
	// Modified is when the corresponding WorkflowRun was last modified.
	Modified string `json:"modified"`
	// Status is the deployment status: one of `not-started`, `accepted`,
	// `running`, `failed`, `succeeded`, `skipped`.
	Status string `json:"status"`
	// Version is the ordinal ID for the stack at the time of this deployment.
	Version int64 `json:"version"`
	// RequestedBy describes the user who created the deployment.
	RequestedBy UserInfo `json:"requestedBy"`
	// ProjectName is the name of the project the stack belongs to.
	ProjectName string `json:"projectName,omitempty"`
	// StackName is the name of the stack.
	StackName string `json:"stackName,omitempty"`
	// Paused is true when deployments are paused for the program.
	Paused bool `json:"paused,omitempty"`
	// PulumiOperation is the Pulumi operation that was performed.
	PulumiOperation PulumiOperation `json:"pulumiOperation"`
	// Updates is the list of stack updates produced by this deployment.
	Updates []DeploymentNestedUpdate `json:"updates"`
	// Jobs is the list of jobs run as part of this deployment, with their
	// step-level progress.
	Jobs []DeploymentJob `json:"jobs"`
	// Initiator records the initiation source of the deployment (e.g.
	// `cli`, `webhook`). Empty when the server did not report one.
	Initiator string `json:"initiator,omitempty"`
	// AgentPool is the self-hosted agent pool that ran the deployment,
	// if any.
	AgentPool *ListDeploymentSnapshotAgentPool `json:"agentPool,omitempty"`
	// InheritSettings indicates whether the deployment inherited
	// deployment settings from the stack at creation time.
	InheritSettings bool `json:"inheritSettings,omitempty"`
}
