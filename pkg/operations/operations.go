// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package operations

import (
	"time"
)

// LogEntry is a row in the logs for a running compute service
type LogEntry struct {
	ID        string
	Timestamp int64
	Message   string
}

// ResourceFilter specifies a specific resource or subset of resources.  It can be provided in three formats:
// - Full URN: "<namespace>::<alloc>::<type>::<name>"
// - Type + Name: "<type>::<name>"
// - Name: "<name>"
type ResourceFilter string

// LogQuery represents the parameters to a log query operation.
// All fields are optional, leaving them off returns all logs.
type LogQuery struct {
	// StartTime is an optional time indiciating that only logs from after this time should be produced.
	StartTime *time.Time
	// EndTime is an optional time indiciating that only logs from before this time should be produced.
	EndTime *time.Time
	// ResourceFilter is a string indicating that logs should be limited to a resource or resources
	ResourceFilter *ResourceFilter
}

// Provider is the interface for making operational requests about the
// state of a Component (or Components)
type Provider interface {
	// GetLogs returns logs matching a query
	GetLogs(query LogQuery) (*[]LogEntry, error)
	// TODO[pulumi/pulumi#609] Add support for metrics
}
