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
	// Query is a string indicating a filter to apply to the logs - query syntax TBD
	Query *string
	// Resource is a string indicating that logs should be limited toa resource of resoruces
	Resource *ResourceFilter
}

// MetricName is a handle to a metric supported by a Pulumi Framework resources
type MetricName string

// MetricRequest is a request for a metric name
type MetricRequest struct {
	Name string
}

// MetricDataPoint is a data point returned from a metric.
type MetricDataPoint struct {
	Timestamp   time.Time
	Unit        string
	Sum         float64
	SampleCount float64
	Average     float64
	Maximum     float64
	Minimum     float64
}

// Provider is the interface for making operational requests about the
// state of a Component (or Components)
type Provider interface {
	// GetLogs returns logs matching a query
	GetLogs(query LogQuery) (*[]LogEntry, error)
	// ListMetrics returns the list of supported metrics for the requested component type
	ListMetrics() []MetricName
	// GetMetricStatistics provides metrics data for a given metric request
	GetMetricStatistics(metric MetricRequest) ([]MetricDataPoint, error)
}
