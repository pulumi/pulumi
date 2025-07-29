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
	JournalEntryBegin          JournalEntryKind = 0
	JournalEntrySuccess        JournalEntryKind = 1
	JournalEntryFailure        JournalEntryKind = 2
	JournalEntryRefreshSuccess JournalEntryKind = 3
	JournalEntryOutputs        JournalEntryKind = 4
	JournalEntryRebase         JournalEntryKind = 5
	JournalEntrySecretsManager JournalEntryKind = 6
)

type JournalEntry struct {
	Kind JournalEntryKind `json:"kind"`
	// ID of the operation this journal entry is associated with.
	OperationID uint64 `json:"operationID"`
	// ID for the delete Operation that this journal entry is associated with.
	DeleteOld int `json:"deleteOld"`
	// ID for the delete Operation that this journal entry is associated with.
	DeleteNew uint64 `json:"deleteNew"`
	// The resource state associated with this journal entry.
	State *ResourceV3 `json:"state,omitempty"`
	// The operation associated with this journal entry, if any.
	Operation *OperationV2 `json:"operation,omitempty"`
	// The secrets manager associated with this journal entry, if any.
	SecretsProvider *SecretsProvidersV1 `json:"secretsProvider,omitempty"`
	// PendingReplacement is the index of the resource that's to be marked aspending replacement.
	PendingReplacement int `json:"pendingReplacement,omitempty"`
	// NewSnapshot is the new snapshot that this journal entry is associated with.
	NewSnapshot *DeploymentV3 `json:"newSnapshot,omitempty"`
}

// CreateJournalEntryRequest defines the request body for creating a new journal entry.
type CreateJournalEntryRequest struct {
	// Data is the JSON blob representing the journal entry.
	Data JournalEntry `json:"data"`
	// UpdateID is an identifier for the update this journal entry is associated with.
	UpdateID string `json:"updateID"`
	// StackID is the identifier for the stack this journal entry is associated with.
	StackID string `json:"stackID"`
}

// CreateJournalEntryResponse defines the response for creating a new journal entry.
type CreateJournalEntryResponse struct {
	// ID is the unique identifier for the created journal entry.
	ID string `json:"id"`
}
