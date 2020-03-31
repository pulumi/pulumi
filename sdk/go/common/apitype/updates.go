// Copyright 2016-2018, Pulumi Corporation.
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

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
)

// CreateUpdateConfig describes the configuration data for an request to `POST /updates`.
type CreateUpdateConfig struct {
	// Contents contains the configuration values for an update as a set of key-value pairs.
	Contents map[string]ConfigValue `json:"contents"`
}

// UpdateProgramRequest is the request type for updating (aka deploying) a Pulumi program.
type UpdateProgramRequest struct {
	// Properties from the Project file. Subset of pack.Package.
	Name        string `json:"name"`
	Runtime     string `json:"runtime"`
	Main        string `json:"main"`
	Description string `json:"description"`

	Options UpdateOptions `json:"options"`

	// Configuration values.
	Config map[string]ConfigValue `json:"config"`

	Metadata UpdateMetadata `json:"metadata"`
}

// UpdateOptions is the set of operations for configuring the output of an update.
//
// Should generally mirror engine.UpdateOptions, but we clone it in this package to add
// flexibility in case there is a breaking change in the engine-type.
type UpdateOptions struct {
	LocalPolicyPackPaths []string            `json:"localPolicyPackPaths"`
	Color                colors.Colorization `json:"color"`
	DryRun               bool                `json:"dryRun"`
	Parallel             int                 `json:"parallel"`
	ShowConfig           bool                `json:"showConfig"`
	ShowReplacementSteps bool                `json:"showReplacementSteps"`
	ShowSames            bool                `json:"showNames"`
	Summary              bool                `json:"summary"`
	Debug                bool                `json:"debug"`
}

// UpdateMetadata describes optional metadata about an update.
//
// Should generally mirror backend.UpdateMetadata, but we clone it in this package to add
// flexibility in case there is a breaking change in the backend-type.
type UpdateMetadata struct {
	// Message is an optional message associated with the update.
	Message string `json:"message"`
	// Environment contains optional data from the deploying environment. e.g. the current
	// source code control commit information.
	Environment map[string]string `json:"environment"`
}

// UpdateProgramResponse is the result of an update program request.
type UpdateProgramResponse struct {
	// UpdateID is the opaque identifier of the requested update. This value is needed to begin an update, as
	// well as poll for its progress.
	UpdateID string `json:"updateID"`

	// RequiredPolicies is a list of required Policy Packs to run during the update.
	RequiredPolicies []RequiredPolicy `json:"requiredPolicies,omitempty"`
}

// StartUpdateRequest requests that an update starts getting applied to a stack.
type StartUpdateRequest struct {
	// Tags contains an updated set of Tags for the stack. If non-nil, will replace the current
	// set of tags associated with the stack.
	Tags map[StackTagName]string `json:"tags,omitempty"`
}

// StartUpdateResponse is the result of the command to start an update.
type StartUpdateResponse struct {
	// Version is the version of the program once the update is complete.
	// (Will be the current, unchanged value for previews.)
	Version int `json:"version"`

	// Token is the lease token (if any) to be used to authorize operations on this update.
	Token string `json:"token,omitempty"`
}

// UpdateEventKind is an enum for the type of update events.
type UpdateEventKind string

const (
	// StdoutEvent is used to mark the event being emitted to STDOUT.
	StdoutEvent UpdateEventKind = "stdout"
	// StderrEvent is used to mark the event being emitted to STDERR.
	StderrEvent UpdateEventKind = "stderr"
)

// UpdateEvent describes an event that happened on the Pulumi Cloud while processing an update.
type UpdateEvent struct {
	Index  string                 `json:"index"`
	Kind   UpdateEventKind        `json:"kind"`
	Fields map[string]interface{} `json:"fields"`
}

// UpdateStatus is an enum describing the current state during the lifecycle of an update.
type UpdateStatus string

const (
	// StatusNotStarted is returned when the Update has been created but not applied.
	StatusNotStarted UpdateStatus = "not started"
	// StatusRequested is returned when the Update application has been requested but not started.
	StatusRequested UpdateStatus = "requested"
	// StatusRunning is returned when the Update is in progress.
	StatusRunning UpdateStatus = "running"
	// StatusFailed is returned when the update has failed.
	StatusFailed UpdateStatus = "failed"
	// StatusSucceeded is returned when the update has succeeded.
	StatusSucceeded UpdateStatus = "succeeded"
)

// UpdateResults returns a series of events and the current status of an update. The events can be filtered. See
// API call for more details.
type UpdateResults struct {
	Status UpdateStatus  `json:"status"`
	Events []UpdateEvent `json:"events"`

	// ContinuationToken is an opaque value used to indiciate the end of the returned update
	// results. Pass it in the next request to obtain subsequent update events.
	//
	// The same continuation token may be returned if no new update events are available, but the
	// update is still in-progress.
	//
	// A value of nil means that no new updates will be available. Everything has been returned to
	// the client and the update has completed.
	ContinuationToken *string `json:"continuationToken,omitempty"`
}

// UpdateProgram describes the metadata associated with an update's Pulumi program. Note that this does not
// include the contents of the program itself.
type UpdateProgram struct {
	// Name is the name of the program.
	Name string `json:"name"`

	// Runtime is the language runtime used to execute the program.
	Runtime string `json:"runtime"`

	// Main is an optional redirect for the main program location. (e.g. a subfolder under Pulumi.yaml
	// containing package.json.)
	Main string `json:"main"`

	// Analyzers is the set of analyzers to run when this program is executed.
	Analyzers []string `json:"analyzers"`

	// Destroy indicates whether or not this program is the nil program (i.e. the program that generates no resources).
	Destroy bool `json:"destroy"`

	// Refresh is true if this is a refresh-style update, which simply takes state from the current cloud resources.
	Refresh bool `json:"refresh"`
}

// RenewUpdateLeaseRequest defines the body of a request to the update lease renewal endpoint of the service API.
type RenewUpdateLeaseRequest struct {
	// The current, valid lease token.
	// DEPRECATED as of Pulumi API version 5+. Pulumi API will expect the update token
	// in the Authorization header instead of this property. This property will be removed
	// when the minimum supported API version on the service is raised to 5.
	Token string `json:"token"`
	// The duration for which to renew the lease in seconds (maximum 300).
	Duration int `json:"duration"`
}

// RenewUpdateLeaseResponse defines the data returned by the update lease renewal endpoint of the service API.
type RenewUpdateLeaseResponse struct {
	// The renewed token.
	Token string `json:"token"`
}

const (
	// UpdateStatusSucceeded indicates that an update completed successfully.
	UpdateStatusSucceeded UpdateStatus = "succeeded"
	// UpdateStatusFailed indicates that an update completed with one or more failures.
	UpdateStatusFailed UpdateStatus = "failed"
	// UpdateStatusCancelled indicates that an update completed due to cancellation.
	UpdateStatusCancelled UpdateStatus = "cancelled"
)

// CompleteUpdateRequest defines the body of a request to the update completion endpoint of the service API.
type CompleteUpdateRequest struct {
	Status UpdateStatus `json:"status"`
}

// PatchUpdateCheckpointRequest defines the body of a request to the patch update checkpoint endpoint of the service
// API. The `Deployment` field is expected to contain a serialized `Deployment` value, the schema of which is indicated
// by the `Version` field.
type PatchUpdateCheckpointRequest struct {
	IsInvalid  bool            `json:"isInvalid"`
	Version    int             `json:"version"`
	Deployment json.RawMessage `json:"deployment,omitempty"`
}

// AppendUpdateLogEntryRequest defines the body of a request to the append update log entry endpoint of the service API.
// No longer sent from the CLI, but the type definition is still required for backwards compat with older clients.
type AppendUpdateLogEntryRequest struct {
	Kind   string                 `json:"kind"`
	Fields map[string]interface{} `json:"fields"`
}

// StackRenameRequest is the shape of the request to change an existing stack's name.
// If either NewName or NewProject is the empty string, the current project/name will
// be preserved. (But at least one should be set.)
type StackRenameRequest struct {
	NewName    string `json:"newName"`
	NewProject string `json:"newProject"`
}
