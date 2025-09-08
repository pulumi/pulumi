// Copyright 2025, Pulumi Corporation.
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

type JournalEntryKind int

const (
	JournalEntryKindBegin          JournalEntryKind = 0
	JournalEntryKindSuccess        JournalEntryKind = 1
	JournalEntryKindFailure        JournalEntryKind = 2
	JournalEntryKindRefreshSuccess JournalEntryKind = 3
	JournalEntryKindOutputs        JournalEntryKind = 4
	JournalEntryKindWrite          JournalEntryKind = 5
	JournalEntryKindSecretsManager JournalEntryKind = 6
)

type JournalEntry struct {
	Kind JournalEntryKind `json:"kind"`
	// Sequence ID of the operation.
	SequenceID int64 `json:"sequenceID"`
	// ID of the operation this journal entry is associated with.
	OperationID int64 `json:"operationID"`
	// ID for the delete Operation that this journal entry is associated with.
	RemoveOld *int64 `json:"deleteOld"`
	// ID for the delete Operation that this journal entry is associated with.
	RemoveNew *int64 `json:"deleteNew"`
	// PendingReplacement is the index of the resource that's to be marked as pending replacement
	PendingReplacement *int64 `json:"pendingReplacement,omitempty"`
	// Delete is the index of the resource that's to be marked as deleted.
	Delete *int64 `json:"delete,omitempty"`
	// The resource state associated with this journal entry.
	State *ResourceV3 `json:"state,omitempty"`
	// The operation associated with this journal entry, if any.
	Operation *OperationV2 `json:"operation,omitempty"`
	// If true, this journal entry is part of a refresh operation.
	IsRefresh bool `json:"isRefresh,omitempty"`
	// The secrets manager associated with this journal entry, if any.
	SecretsProvider *SecretsProvidersV1 `json:"secretsProvider,omitempty"`

	// NewSnapshot is the new snapshot that this journal entry is associated with.
	NewSnapshot *DeploymentV3 `json:"newSnapshot,omitempty"`
}

type JournalEntries struct {
	Entries []JournalEntry `json:"entries,omitempty"`
}
