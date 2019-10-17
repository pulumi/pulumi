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

package operations

import (
	"time"
)

// LogEntry is a row in the logs for a running compute service
type LogEntry struct {
	ID string
	// Timestamp is a Unix timestamp, in milliseconds
	Timestamp int64
	Message   string
}

// ResourceFilter specifies a specific resource or subset of resources.  It can be provided in three formats:
// - Full URN: "<namespace>::<alloc>::<type>::<name>"
// - Type + Name: "<type>::<name>"
// - Name: "<name>"
type ResourceFilter string

// LogQuery represents the parameters to a log query operation. All fields are
// optional, leaving them off returns all logs.
//
// IDEA: We are currently using this type both within the engine and as an
// apitype. We should consider splitting this into separate types for the engine
// and on the wire.
type LogQuery struct {
	// StartTime is an optional time indiciating that only logs from after this time should be produced.
	StartTime *time.Time `url:"startTime,unix"`
	// EndTime is an optional time indiciating that only logs from before this time should be produced.
	EndTime *time.Time `url:"endTime,unix"`
	// ResourceFilter is a string indicating that logs should be limited to a resource or resources
	ResourceFilter *ResourceFilter `url:"resourceFilter"`
}

// Provider is the interface for making operational requests about the
// state of a Component (or Components)
type Provider interface {
	// GetLogs returns logs matching a query
	GetLogs(query LogQuery) (*[]LogEntry, error)
	// TODO[pulumi/pulumi#609] Add support for metrics
}
