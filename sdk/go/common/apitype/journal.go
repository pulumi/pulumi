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

import (
	"fmt"
	"strings"
)

type JournalEntryKind int

const (
	JournalEntryKindBegin            JournalEntryKind = 0
	JournalEntryKindSuccess          JournalEntryKind = 1
	JournalEntryKindFailure          JournalEntryKind = 2
	JournalEntryKindRefreshSuccess   JournalEntryKind = 3
	JournalEntryKindOutputs          JournalEntryKind = 4
	JournalEntryKindWrite            JournalEntryKind = 5
	JournalEntryKindSecretsManager   JournalEntryKind = 6
	JournalEntryKindRebuiltBaseState JournalEntryKind = 7
)

func (k JournalEntryKind) String() string {
	switch k {
	case JournalEntryKindBegin:
		return "begin"
	case JournalEntryKindSuccess:
		return "success"
	case JournalEntryKindFailure:
		return "failure"
	case JournalEntryKindRefreshSuccess:
		return "refresh-success"
	case JournalEntryKindOutputs:
		return "outputs"
	case JournalEntryKindWrite:
		return "write"
	case JournalEntryKindSecretsManager:
		return "secrets-manager"
	case JournalEntryKindRebuiltBaseState:
		return "rebuilt-base-state"
	default:
		return "invalid"
	}
}

type JournalEntry struct {
	// Version of the journal entry format.
	Version int `json:"version"`
	// Kind of journal entry.
	Kind JournalEntryKind `json:"kind"`
	// Sequence ID of the operation.
	SequenceID int64 `json:"sequenceID"`
	// ID of the operation this journal entry is associated with.
	OperationID int64 `json:"operationID"`
	// ID for the delete Operation that this journal entry is associated with.
	RemoveOld *int64 `json:"removeOld"`
	// ID for the delete Operation that this journal entry is associated with.
	RemoveNew *int64 `json:"removeNew"`
	// PendingReplacementOld is the index of the resource that's to be marked as pending replacement
	PendingReplacementOld *int64 `json:"pendingReplacementOld,omitempty"`
	// PendingReplacementNew is the operation ID of the new resource to be marked as pending replacement
	PendingReplacementNew *int64 `json:"pendingReplacementNew,omitempty"`
	// DeleteOld is the index of the resource that's to be marked as deleted.
	DeleteOld *int64 `json:"deleteOld,omitempty"`
	// DeleteNew is the operation ID of the new resource to be marked as deleted.
	DeleteNew *int64 `json:"deleteNew,omitempty"`
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

func (e JournalEntry) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "(%v, %v): %v", e.SequenceID, e.OperationID, e.Kind)
	if e.RemoveOld != nil {
		fmt.Fprintf(&sb, ", removeOld:%v", *e.RemoveOld)
	}
	if e.RemoveNew != nil {
		fmt.Fprintf(&sb, ", removeNew:%v", *e.RemoveNew)
	}
	if e.PendingReplacementOld != nil {
		fmt.Fprintf(&sb, ", removePendingReplacementOld:%v", *e.PendingReplacementOld)
	}
	if e.PendingReplacementNew != nil {
		fmt.Fprintf(&sb, ", removePendingReplacementNew:%v", *e.PendingReplacementNew)
	}
	if e.DeleteOld != nil {
		fmt.Fprintf(&sb, ", deleteOld:%v", *e.DeleteOld)
	}
	if e.DeleteNew != nil {
		fmt.Fprintf(&sb, ", deleteNew:%v", *e.DeleteNew)
	}
	if e.State != nil {
		fmt.Fprintf(&sb, ", state(%v)", e.State.URN)
	}
	if e.Operation != nil {
		fmt.Fprintf(&sb, ", operation(%v)", e.Operation.Type)
	}
	if e.IsRefresh {
		fmt.Fprintf(&sb, ", isRefresh")
	}
	if e.SecretsProvider != nil {
		fmt.Fprintf(&sb, ", secretsProvider(%v)", e.SecretsProvider.Type)
	}
	if e.NewSnapshot != nil {
		fmt.Fprintf(&sb, ", +snap")
	}

	return sb.String()
}

type JournalEntries struct {
	Entries []JournalEntry `json:"entries,omitempty"`
}
