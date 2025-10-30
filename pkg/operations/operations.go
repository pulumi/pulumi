package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/operations"

// LogEntry is a row in the logs for a running compute service
type LogEntry = operations.LogEntry

// ResourceFilter specifies a specific resource or subset of resources.  It can be provided in three formats:
// - Full URN: "<namespace>::<alloc>::<type>::<name>"
// - Type + Name: "<type>::<name>"
// - Name: "<name>"
type ResourceFilter = operations.ResourceFilter

// LogQuery represents the parameters to a log query operation. All fields are
// optional, leaving them off returns all logs.
// 
// IDEA: We are currently using this type both within the engine and as an
// apitype. We should consider splitting this into separate types for the engine
// and on the wire.
type LogQuery = operations.LogQuery

// Provider is the interface for making operational requests about the
// state of a Component (or Components)
type Provider = operations.Provider

