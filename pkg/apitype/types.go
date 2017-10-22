package apitype

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/tokens"
)

/**
 * Go type declarations for REST objects returned from the Pulumi Console API.
 */

// User represents a Pulumi user.
type User struct {
	ID            string        `json:"id"`
	GitHubLogin   string        `json:"githubLogin"`
	Name          string        `json:"name"`
	AvatarURL     string        `json:"avatarUrl"`
	Organizations []interface{} `json:"organizations"`
}

// ErrorResponse is returned from the API when an actual response body is not appropriate. i.e.
// in all error situations.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the Error interface.
func (err ErrorResponse) Error() string {
	return fmt.Sprintf("[%d] %s", err.Code, err.Message)
}

// UpdateProgramRequest is the request type for updating (aka deploying) a Pulumi program.
type UpdateProgramRequest struct {
	// Base-64 encoded Zip archive of the program's root directory.
	ProgramArchive string `json:"programArchive"`

	// Configuration values.
	Config map[tokens.ModuleMember]string `json:"config"`
}

// UpdateProgramResponse is the response type when updating a Pulumi program.
type UpdateProgramResponse struct {
	Version int `json:"version"`
}

// UpdateEvent Kind values.
const (
	// StdoutEvent is used to mark the event being emitted to STDOUT.
	StdoutEvent = "stdout"
	// StderrEvent is used to mark the event being emitted to STDERR.
	StderrEvent = "stderr"
)

// UpdateEvent describes an event that happened on the Pulumi Cloud while processing an update.
type UpdateEvent struct {
	Index  int                    `json:"index"`
	Kind   string                 `json:"kind"`
	Fields map[string]interface{} `json:"fields"`
}

// UpdateResults Status field values.
const (
	// StatusNotStarted is returned when the Update has been created but not applied.
	StatusNotStarted = "not started"
	// StatusRequested is returned when the Update application has been requested but not started.
	StatusRequested = "requested"
	// StatusRunning is returned when the Update is in progress.
	StatusRunning = "running"
	// StatusFailed is returned when the update has failed.
	StatusFailed = "failed"
	// StatusSucceeded is returned when the update has succeeded.
	StatusSucceeded = "succeeded"
)

// UpdateResults returns a series of events and the current status of an update. The vents can
// be filtered. See API call for more details.
type UpdateResults struct {
	Status string        `json:"status"`
	Events []UpdateEvent `json:"events"`
}
