// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package apitype contains the type definitions for JSON objects returned from the Pulumi Cloud
// Console's REST API. Thes
package apitype

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/tokens"
)

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
	// Properties from the Project file.
	Name    tokens.PackageName `json:"name"`
	Runtime string             `json:"runtime"`

	// Base-64 encoded Zip archive of the program's root directory.
	ProgramArchive string `json:"programArchive"`

	// Configuration values.
	Config map[tokens.ModuleMember]string `json:"config"`
}

// PreviewUpdateResponse is returned when previewing a potential update.
type PreviewUpdateResponse struct {
	UpdateID string `json:"updateID"`
}

// UpdateProgramResponse is the response type when updating a Pulumi program.
type UpdateProgramResponse struct {
	UpdateID string `json:"updateID"`
	// Version is the program's new version being updated to.
	Version int `json:"version"`
}

// DestroyProgramResponse is the response type when destroying a Pulumi program's resources.
type DestroyProgramResponse struct {
	UpdateID string `json:"updateID"`
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
