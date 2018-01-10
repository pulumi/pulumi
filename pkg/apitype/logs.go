// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

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
