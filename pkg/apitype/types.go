// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package apitype contains the type definitions for JSON objects returned from the Pulumi Cloud
// Console's REST API. Thes
package apitype

import (
	"fmt"
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

// LogsResult is the JSON shape of responses to a Logs operation.
type LogsResult struct {
	Logs []LogEntry `json:"logs"`
}

// LogEntry is the individual entries in a JSON response to a Logs operation.
type LogEntry struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}
