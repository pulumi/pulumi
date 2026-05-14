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

import "encoding/json"

// ScheduledActionKind identifies what kind of action a scheduled action executes.
type ScheduledActionKind string

const (
	// ScheduledActionKindDeployment is a scheduled deployment action.
	ScheduledActionKindDeployment ScheduledActionKind = "deployment"
)

// ScheduledAction describes the state of a scheduled action returned by the Pulumi Cloud REST API.
type ScheduledAction struct {
	// ID is the unique identifier for this scheduled action.
	ID string `json:"id"`
	// OrgID is the organization ID that owns this scheduled action.
	OrgID string `json:"orgID,omitempty"`
	// ScheduleCron is a cron expression defining the recurring schedule.
	ScheduleCron string `json:"scheduleCron,omitempty"`
	// ScheduleOnce is a timestamp for a one-time scheduled execution.
	ScheduleOnce string `json:"scheduleOnce,omitempty"`
	// NextExecution is the timestamp of the next scheduled execution.
	NextExecution string `json:"nextExecution,omitempty"`
	// Paused indicates whether the scheduled action is currently paused.
	Paused bool `json:"paused"`
	// Kind is the kind of action to be executed.
	Kind ScheduledActionKind `json:"kind"`
	// Definition is the action definition, which varies based on the action kind.
	Definition json.RawMessage `json:"definition,omitempty"`
	// Created is the timestamp when this scheduled action was created.
	Created string `json:"created,omitempty"`
	// Modified is the timestamp when this scheduled action was last modified.
	Modified string `json:"modified,omitempty"`
	// LastExecuted is the timestamp of the last execution, if any.
	LastExecuted *string `json:"lastExecuted,omitempty"`
}

// ListScheduledActionsResponse is the API response when scheduled actions are listed.
type ListScheduledActionsResponse struct {
	Schedules []ScheduledAction `json:"schedules"`
}

// ScheduledDeploymentDefinition is the shape of ScheduledAction.Definition when Kind is
// ScheduledActionKindDeployment.
type ScheduledDeploymentDefinition struct {
	// ProgramID is the Pulumi Cloud-internal identifier for the program (stack) being deployed.
	ProgramID string `json:"programID,omitempty"`
	// Request is the deployment request payload that will be executed when the schedule fires.
	Request *CreateDeploymentRequest `json:"request,omitempty"`
}
