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

package apitype

import "encoding/json"

// UpdateKind is an enum for the type of update performed.
//
// Should generally mirror backend.UpdateKind, but we clone it in this package to add
// flexibility in case there is a breaking change in the backend-type.
type UpdateKind string

const (
	// UpdateUpdate is the prototypical Pulumi program update.
	UpdateUpdate UpdateKind = "update"
	// PreviewUpdate is a preview of an update, without impacting resources.
	PreviewUpdate UpdateKind = "preview"
	// RefreshUpdate is an update that came from a refresh operation.
	RefreshUpdate UpdateKind = "refresh"
	// RenameUpdate is an update that changes the stack name or project name of a Pulumi program.
	RenameUpdate UpdateKind = "rename"
	// DestroyUpdate is an update which removes all resources.
	DestroyUpdate UpdateKind = "destroy"
	// StackImportUpdate is an update that entails importing a raw checkpoint file.
	StackImportUpdate UpdateKind = "import"
	// ResourceImportUpdate is an update that entails importing one or more resources.
	ResourceImportUpdate = "resource-import"
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
	// OpSame indicates no change was made.
	OpSame OpType = "same"
	// OpCreate indicates a new resource was created.
	OpCreate OpType = "create"
	// OpUpdate indicates an existing resource was updated.
	OpUpdate OpType = "update"
	// OpDelete indicates an existing resource was deleted.
	OpDelete OpType = "delete"
	// OpReplace indicates an existing resource was replaced with a new one.
	OpReplace OpType = "replace"
	// OpCreateReplacement indicates a new resource was created for a replacement.
	OpCreateReplacement OpType = "create-replacement"
	// OpDeleteReplaced indicates an existing resource was deleted after replacement.
	OpDeleteReplaced OpType = "delete-replaced"
	// OpRead indicates reading an existing resource.
	OpRead OpType = "read"
	// OpReadReplacement indicates reading an existing resource for a replacement.
	OpReadReplacement OpType = "read-replacement"
	// OpRefresh indicates refreshing an existing resource.
	OpRefresh OpType = "refresh" // refreshing an existing resource.
	// OpReadDiscard indicates removing a resource that was read.
	OpReadDiscard OpType = "discard"
	// OpDiscardReplaced indicates discarding a read resource that was replaced.
	OpDiscardReplaced OpType = "discard-replaced"
	// OpRemovePendingReplace indicates removing a pending replace resource.
	OpRemovePendingReplace OpType = "remove-pending-replace"
	// OpImport indicates importing an existing resource.
	OpImport OpType = "import"
	// OpImportReplacement indicates replacement of an existing resource with an imported resource.
	OpImportReplacement OpType = "import-replacement"
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
	Result          UpdateResult    `json:"result"`
	EndTime         int64           `json:"endTime"`
	Version         int             `json:"version"`
	Deployment      json.RawMessage `json:"deployment,omitempty"`
	ResourceChanges map[OpType]int  `json:"resourceChanges,omitempty"`
}

// GetHistoryResponse is the response from the Pulumi Service when requesting
// a stack's history.
type GetHistoryResponse struct {
	Updates []UpdateInfo `json:"updates"`
}
