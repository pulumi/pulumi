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

// ScheduledAction is the response shape for a single Pulumi ESC scheduled
// action. Fields mirror the `ScheduledAction` OpenAPI schema in
// `pulumi-service/pkg/apitype/spec/openapi_public.json`.
//
// Kind is one of the values enumerated by the server: "deployment",
// "environment_rotation", "scan", "agent_automation". The Definition field
// carries action-specific payload — modelled as an open map so we don't have
// to chase server-side additions.
type ScheduledAction struct {
	ID            string         `json:"id"`
	OrgID         string         `json:"orgID"`
	Kind          string         `json:"kind"`
	ScheduleCron  string         `json:"scheduleCron,omitempty"`
	ScheduleOnce  string         `json:"scheduleOnce,omitempty"`
	Paused        bool           `json:"paused"`
	Created       string         `json:"created"`
	Modified      string         `json:"modified"`
	LastExecuted  string         `json:"lastExecuted"`
	NextExecution string         `json:"nextExecution"`
	Definition    map[string]any `json:"definition,omitempty"`
}

// ListScheduledActionsResponse is returned by the list endpoint.
type ListScheduledActionsResponse struct {
	Schedules []ScheduledAction `json:"schedules"`
}

// CreateEnvironmentScheduleRequest is the POST body for environment schedules.
// Set exactly one of ScheduleCron or ScheduleOnce; provide a SecretRotationRequest
// for the action payload.
type CreateEnvironmentScheduleRequest struct {
	ScheduleCron          string                                          `json:"scheduleCron,omitempty"`
	ScheduleOnce          string                                          `json:"scheduleOnce,omitempty"`
	SecretRotationRequest *CreateEnvironmentSecretRotationScheduleRequest `json:"secretRotationRequest,omitempty"`
}

// CreateEnvironmentSecretRotationScheduleRequest describes the secret-rotation
// action. The OpenAPI schema requires `environmentPath`; an empty string means
// "rotate every rotated secret in the environment".
type CreateEnvironmentSecretRotationScheduleRequest struct {
	// EnvironmentPath optionally narrows rotation to a single rotated secret.
	// Leave empty to rotate all rotated secrets in the environment.
	EnvironmentPath string `json:"environmentPath"`
}
