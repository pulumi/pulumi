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
	"sort"
	"time"

	common "go.opentelemetry.io/proto/otlp/common/v1"
	logs "go.opentelemetry.io/proto/otlp/logs/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	resources "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ResourceFilter specifies a specific resource or subset of resources.  It can be provided in three formats:
// - Full URN: "<namespace>::<alloc>::<type>::<name>"
// - Type + Name: "<type>::<name>"
// - Name: "<name>"
type ResourceFilter = string

// LogQuery represents the parameters to a log query operation. All fields are
// optional, leaving them off returns all logs.
type LogQuery struct {
	// StartTime is an optional time indiciating that only logs from after this time should be produced.
	StartTime *time.Time
	// EndTime is an optional time indiciating that only logs from before this time should be produced.
	EndTime *time.Time
	// ResourceFilter is a string indicating that logs should be limited to a resource or resources
	ResourceFilter ResourceFilter
	// Count is the number of log entries to return.
	Count int
	// ContinuationToken is the continuation token (if any) returned by the last call to GetLogs.
	ContinuationToken interface{}
}

// MetricsQuery represents the parameters to a metrics query operation. All fields are
// optional, leaving them off returns all metrics.
type MetricsQuery struct {
	// StartTime is an optional time indiciating that only metrics from after this time should be produced.
	StartTime *time.Time
	// EndTime is an optional time indiciating that only metrics from before this time should be produced.
	EndTime *time.Time
	// ResourceFilter is a string indicating that metrics should be limited to a resource or resources
	ResourceFilter ResourceFilter
	// Count is the number of metric datapoints to return.
	Count int
	// ContinuationToken is the continuation token (if any) returned by the last call to GetMetrics.
	ContinuationToken interface{}
}

// Provider is the interface for making operational requests about the
// state of a Component (or Components)
type Provider interface {
	// GetLogs returns logs matching a query
	GetLogs(query LogQuery) ([]*logs.ResourceLogs, interface{}, error)
	// GetMetrics returns metrics matching a query
	GetMetrics(query MetricsQuery) ([]*metrics.ResourceMetrics, interface{}, error)
}

// LogEntry is a row in the logs for a running compute service
type LogEntry struct {
	URN resource.URN
	ID  resource.ID

	// Timestamp is a Unix timestamp, in milliseconds
	Timestamp int64
	Message   string
}

// PivotLogs pivots a set of resource logs into a set of log entries sorted by timestamp.
func PivotLogs(resourceLogs []*logs.ResourceLogs) []LogEntry {
	var result []LogEntry
	for _, r := range resourceLogs {
		urn, id := UnpackResource(r.Resource)
		for _, s := range r.ScopeLogs {
			for _, l := range s.LogRecords {
				result = append(result, LogEntry{
					URN:       urn,
					ID:        id,
					Timestamp: int64(l.TimeUnixNano) / 1000000,
					Message:   l.Body.GetStringValue(),
				})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp < result[j].Timestamp })
	return result
}

func PackResource(urn resource.URN, id resource.ID) *resources.Resource {
	attrs := []*common.KeyValue{{
		Key: "urn",
		Value: &common.AnyValue{
			Value: &common.AnyValue_StringValue{StringValue: string(urn)},
		},
	}}
	if id != "" {
		attrs = append(attrs, &common.KeyValue{
			Key: "id",
			Value: &common.AnyValue{
				Value: &common.AnyValue_StringValue{StringValue: string(id)},
			},
		})
	}
	return &resources.Resource{Attributes: attrs}
}

func UnpackResource(r *resources.Resource) (urn resource.URN, id resource.ID) {
	for _, attr := range r.Attributes {
		switch attr.Key {
		case "urn":
			urn = resource.URN(attr.Value.GetStringValue())
		case "id":
			id = resource.ID(attr.Value.GetStringValue())
		}
	}
	return
}
