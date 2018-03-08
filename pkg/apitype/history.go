// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

// UpdateKind is an enum for the type of update performed.
//
// Should generally mirror backend.UpdateKind, but we clone it in this package to add
// flexibility in case there is a breaking change in the backend-type.
type UpdateKind string

const (
	// DeployUpdate is the prototypical Pulumi program update.
	DeployUpdate UpdateKind = "update"
	// PreviewUpdate is a preview of an update, without impacting resources.
	PreviewUpdate UpdateKind = "preview"
	// DestroyUpdate is an update which removes all resources.
	DestroyUpdate UpdateKind = "destroy"
)

// UpdateResult is an enum for the result of the update.
//
// Should generally mirror backend.UpdateResult, but we clone it in this package to add
// flexibility in case there is a breaking change in the backend-type.
type UpdateResult string

const (
	// NotStartedResult is for updates that have not started.
	NotStartedResult UpdateResult = "not-started"
	// InProgressResult is for updates that have not yet completed.
	InProgressResult UpdateResult = "in-progress"
	// SucceededResult is for updates that completed successfully.
	SucceededResult UpdateResult = "succeeded"
	// FailedResult is for updates that have failed.
	FailedResult UpdateResult = "failed"
)

// OpType describes the type of operation performed to a resource managed by Pulumi.
//
// Should generally mirror deploy.StepOp, but we clone it in this package to add
// flexibility in case there is a breaking change in the backend-type.
type OpType string

const (
	// OpSame indiciates no change was made.
	OpSame OpType = "same"
	// OpCreate indiciates a new resource was created.
	OpCreate OpType = "create"
	// OpUpdate indicates an existing resource was updated.
	OpUpdate OpType = "update"
	// OpDelete indiciates an existing resource was deleted.
	OpDelete OpType = "delete"
	// OpReplace indicates an existing resource was replaced with a new one.
	OpReplace OpType = "replace"
	// OpCreateReplacement indiciates a new resource was created for a replacement.
	OpCreateReplacement OpType = "create-replacement"
	// OpDeleteReplaced indiciates an existing resource was deleted after replacement.
	OpDeleteReplaced OpType = "delete-replaced"
)

// UpdateInfo describes a previous update.
//
// Should generally mirror backend.UpdateInfo, but we clone it in this package to add
// flexibility in case there is a breaking change in the backend-type.
type UpdateInfo struct {
	// Information known before an update is started.
	Kind        UpdateKind             `json:"kind"`
	StartTime   int64                  `json:"startTime"`
	Message     string                 `json:"message"`
	Environment map[string]string      `json:"environment"`
	Config      map[string]ConfigValue `json:"config"`

	// Information obtained from an update completing.
	Result          UpdateResult   `json:"result"`
	EndTime         int64          `json:"endTime"`
	Deployment      *Deployment    `json:"deployment,omitempty"`
	ResourceChanges map[OpType]int `json:"resourceChanges,omitempty"`
}

// GetHistoryResponse is the response from the Pulumi Service when requesting
// a stack's history.
type GetHistoryResponse struct {
	Updates []UpdateInfo `json:"updates"`
}
