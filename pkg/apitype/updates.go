// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

import (
	"github.com/pulumi/pulumi/pkg/diag/colors"
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
	Analyzers            []string            `json:"analyzers"`
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

// UpdateProgramRequestUntyped is a legacy type: see comment in pulumi-service stacks_update.go
// unmarshalConfig()
// TODO(#478): remove support for string-only config.
type UpdateProgramRequestUntyped struct {
	// Properties from the Project file.
	Name        string `json:"name"`
	Runtime     string `json:"runtime"`
	Main        string `json:"main"`
	Description string `json:"description"`

	// Configuration values. Note that although the element type of this map is an `interface{}`, the value
	// must be either a string or a ConfigValue.
	Config map[string]interface{} `json:"config"`
}

// UpdateProgramResponse is the result of an update program request.
type UpdateProgramResponse struct {
	// UpdateID is the opaque identifier of the requested update. This value is needed to begin an update, as
	// well as poll for its progress.
	UpdateID string `json:"updateID"`

	// UploadURL is a URL the client can use to upload their program's contents into. Ignored for destroys.
	UploadURL string `json:"uploadURL"`
}

// StartUpdateResponse is the result of the command to start an update.
type StartUpdateResponse struct {
	// Version is the version of the program once the update is complete.
	// (Will be the current, unchanged value for previews.)
	Version int `json:"version"`
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

	// Destroy indicates whether or not this program is the nil program (i.e. the program that generates no
	// resources).
	Destroy bool `json:"destroy"`
}

// CreateUpdateRequest describe the data provided as the body of a request to the `POST /updates` endpoint of
// the PPC API.
type CreateUpdateRequest struct {
	// Stack is the unqique ID for the stack that this update targets.
	Stack string `json:"stack"`

	// Import indicates whether or not this update's resources are given by a checkpoint to import rather than
	// an actual Pulumi program.  If this field is `true`, the client must upload the checkpoint file to the
	// URL returned in the response. Config should be empty, as it will be copied from the base update. Program
	// should also be empty, as it will not be used.
	IsCheckpointImport bool `json:"import,omitempty"`

	// StackAlias is the friendly name for the update's stack that will be exposed to the update's Pulumi
	// program.
	StackAlias string `json:"stackAlias,omitempty"`

	// Config records the configuration values for an update. Must be nil if IsCheckpointImport is true.
	Config *CreateUpdateConfig `json:"config,omitempty"`

	// Program records the program metadata for an update. Must be nil if IsCheckpointImport is true.
	Program *UpdateProgram `json:"program,omitempty"`
}

// CreateUpdateResponse describes the data returned by a request to the `POST /updates` endpoint of the PPC
// API.
type CreateUpdateResponse struct {
	// ID is the unique identifier of the newly-created update.
	ID string `json:"id"`

	// Stack is the unique identifier of the stack targeted by the update.
	Stack string `json:"stack"`

	// BaseUpdate is the unique identifier of the update that was active in the stack indicated above at the
	// time at which this update was created.
	BaseUpdate string `json:"baseUpdate"`

	// UploadURL is a URL that the client must use to upload the contents of the program associated with this
	// update. The client should upload the program by sending a `PUT` request to this URL with the contents of
	// the program as a ZIP file in the request body. The `PUT` request must also set the `Content-Length`
	// header.
	UploadURL string `json:"uploadURL"`
}

// UpdateApplyRequest describes the data provided as the body of a request to the `POST
// /updates/{updateID}/apply` and `POST /updates/{updateID}/preview` endpoints of the PPC API.
type UpdateApplyRequest struct {
	// Should we tell the engine to emit information about the configuration during this update.
	ShowConfig bool `json:"showConfig,omitempty"`

	// Should we tell the engine to emit information about resources that have not changed during this update.
	ShowSames bool `json:"showSames,omitempty"`

	// Should we tell the engine to emit information about replacement steps during this update.
	ShowReplacementSteps bool `json:"showReplacementSteps,omitempty"`

	// Should we tell the engine to emit summary information during this update.
	Summary bool `json:"summary,omitempty"`
}

// GetApplyUpdateResultsResponse describes the data returned by the `GET /updates/{updateID}/apply` endpoint of
// the PPC API.
type GetApplyUpdateResultsResponse UpdateResults

// GetPreviewUpdateResultsResponse describes the data returned by the `GET /updates/{updateID}/preview`
// endpoint of the PPC API.
type GetPreviewUpdateResultsResponse UpdateResults
