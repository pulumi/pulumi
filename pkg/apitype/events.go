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

// The "engine events" defined here are a fork of the types and enums defined in the engine
// package. The duplication is intentional to insulate the Pulumi service from various kinds of
// breaking changes.
//
// The types aren't versioned in the same manner as Resource, Deployment, and Checkpoint (see
// apitype/migrate). So care must be taken if these are ever returned from the service to the CLI.

// CancelEvent is emitted when the user initiates a cancellation of the update in progress, or
// the update successfully completes.
type CancelEvent struct{}

// StdoutEngineEvent is emitted whenever a generic message is written, for example warnings
// from the pulumi CLI itself. Less common that DiagnosticEvent
type StdoutEngineEvent struct {
	Message string `json:"message"`
	Color   string `json:"color"`
}

// DiagnosticEvent is emitted whenever a diagnostic message is provided, for example errors from
// a cloud resource provider while trying to create or update a resource.
type DiagnosticEvent struct {
	URN     string `json:"urn"`
	Prefix  string `json:"prefix"`
	Message string `json:"message"`
	Color   string `json:"color"`
	// Severity is one of "info", "info#err", "warning", or "error".
	Severity  string `json:"severity"`
	StreamID  int    `json:"streamID"`
	Ephemeral bool   `json:"ephemeral"`
}

// PreludeEvent is emitted at the start of an update.
type PreludeEvent struct {
	// Config contains the keys and values for the update.
	// Encrypted configuration values may be blinded.
	Config map[string]string `json:"config"`
}

// SummaryEvent is emitted at the end of an update, with a summary of the changes made.
type SummaryEvent struct {
	// MaybeCorrupt is set if one or more of the resources is in an invalid state.
	MaybeCorrupt bool `json:"maybeCorrupt"`
	// Duration is the number of seconds the update was executing.
	DurationSeconds int `json:"durationSeconds"`
	// ResourceChanges contains the count for resource change by type. The keys are deploy.StepOp,
	// which is not exported in this package.
	ResourceChanges map[string]int `json:"resourceChanges"`
}

// StepEventMetadata describes a "step" within the Pulumi engine, which is any concrete action
// to migrate a set of cloud resources from one state to another.
type StepEventMetadata struct {
	// Op is the operation being performed, a deploy.StepOp.
	Op   string `json:"op"`
	URN  string `json:"urn"`
	Type string `json:"type"`

	// Old is the state of the resource before performing the step.
	Old *StepEventStateMetadata `json:"old"`
	// New is the state of the resource after performing the step.
	New *StepEventStateMetadata `json:"new"`
	// Res is the current state of the resource as known. (e.g. equal to old or new depending on
	// circumstance.)
	Res *StepEventStateMetadata `json:"res"`

	// Keys causing a replacement (only applicable for "create" and "replace" Ops).
	Keys []string `json:"keys,omitempty"`
	// Logical is set if the step is a logical operation in the program.
	Logical bool `json:"logical"`
	// Provider actually performing the step.
	Provider string `json:"provider"`
}

// StepEventStateMetadata is the more detailed state information for a resource as it relates to
// a step(s) being performed.
type StepEventStateMetadata struct {
	Type string `json:"type"`
	URN  string `json:"urn"`

	// Custom indicates if the resource is managed by a plugin.
	Custom bool `json:"custom"`
	// Delete is true when the resource is pending deletion due to a replacement.
	Delete bool `json:"delete"`
	// ID is the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
	ID string `json:"id"`
	// Parent is an optional parent URN that this resource belongs to.
	Parent string `json:"parent"`
	// Protect is true to "protect" this resource (protected resources cannot be deleted).
	Protect bool `json:"protect"`
	// Inputs contains the resource's input properties (as specified by the program). Secrets have
	// filtered out, and large assets have been replaced by hashes as applicable.
	Inputs map[string]interface{} `json:"inputs"`
	// Outputs contains the resource's complete output state (as returned by the resource provider).
	Outputs map[string]interface{} `json:"outputs"`
	// Provider is the resource's provider reference
	Provider string `json:"provider"`
	// InitErrors is the set of errors encountered in the process of initializing resource.
	InitErrors []string `json:"initErrors"`
}

// ResourcePreEvent is emitted before a resource is modified.
type ResourcePreEvent struct {
	Metadata StepEventMetadata `json:"metadata"`
	Planning bool              `json:"planning"`
}

// ResOutputsEvent is emitted when a resource is finished being provisioned.
type ResOutputsEvent struct {
	Metadata StepEventMetadata `json:"metadata"`
	Planning bool              `json:"planning"`
}

// ResOpFailedEvent is emitted when a resource operation fails. Typically a DiagnosticEvent is
// emitted before this event, indiciating what the root cause of the error.
type ResOpFailedEvent struct {
	Metadata StepEventMetadata `json:"metadata"`
	Status   int               `json:"status"`
	Steps    int               `json:"steps"`
}

// EngineEvent describes a Pulumi engine event, such as a change to a resource or diagnostic
// message. EngineEvent is a discriminated union of all possible event types, and exactly one
// field will be non-nil.
type EngineEvent struct {
	CancelEvent      *CancelEvent       `json:"cancelEvent,omitempty"`
	StdoutEvent      *StdoutEngineEvent `json:"stdoutEvent,omitempty"`
	DiagnosticEvent  *DiagnosticEvent   `json:"diagnosticEvent,omitempty"`
	PreludeEvent     *PreludeEvent      `json:"preludeEvent,omitempty"`
	SummaryEvent     *SummaryEvent      `json:"summaryEvent,omitempty"`
	ResourcePreEvent *ResourcePreEvent  `json:"resourcePreEvent,omitempty"`
	ResOutputsEvent  *ResOutputsEvent   `json:"resOutputsEvent,omitempty"`
	ResOpFailedEvent *ResOpFailedEvent  `json:"resOpFailedEvent,omitempty"`
}
