package apitype

import "github.com/pulumi/pulumi/pkg/tokens"

// ConfigValue describes a single (possibly secret) configuration value.
type ConfigValue struct {
	// String is either the plaintext value (for non-secrets) or the base64-encoded ciphertext (for secrets).
	String string `json:"string"`
	// Secret is true if this value is a secret and false otherwise.
	Secret bool `json:"secret"`
}

// UpdateProgramRequest is the request type for updating (aka deploying) a Pulumi program.
type UpdateProgramRequest struct {
	// Properties from the Project file. Subset of pack.Package.
	Name        tokens.PackageName `json:"name"`
	Runtime     string             `json:"runtime"`
	Main        string             `json:"main"`
	Description string             `json:"description"`

	// Configuration values.
	Config map[tokens.ModuleMember]ConfigValue `json:"config"`
}

/*
// ConfigValue describes a single (possibly secret) configuration value.
type ConfigValue struct {
	// String is either the plaintext value (for non-secrets) or the base64-encoded ciphertext (for secrets).
	String string `json:"string"`
	// Secret is true if this value is a secret and false otherwise.
	Secret bool `json:"secret"`
}

// UpdateProgramRequest is the request type for updating (aka deploying) a Pulumi program.
type UpdateProgramRequest struct {
	// Properties from the Project file.
	Name        string `json:"name"`
	Runtime     string `json:"runtime"`
	Main        string `json:"main"`
	Description string `json:"description"`

	// Configuration values. Note that although the element type of this map is an `interface{}`, the value must be
	// either a string or a ConfigValue.
	Config map[string]interface{} `json:"config"`
}
*/

// UpdateProgramResponse is the result of an update program request.
type UpdateProgramResponse struct {
	// UpdateID is the opaque identifier of the requested update. This value is needed to begin
	// an update, as well as poll for its progress.
	UpdateID string `json:"updateID"`

	// UploadURL is a URL the client can use to upload their program's contents into. Ignored
	// for destroys.
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
	Kind   string                 `json:"kind"`
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

// UpdateResults returns a series of events and the current status of an update. The vents can
// be filtered. See API call for more details.
type UpdateResults struct {
	Status string        `json:"status"`
	Events []UpdateEvent `json:"events"`
}
