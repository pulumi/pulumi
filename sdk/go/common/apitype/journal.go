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
	JournalEntryKindBegin                 JournalEntryKind = 0
	JournalEntryKindSuccess               JournalEntryKind = 1
	JournalEntryKindFailure               JournalEntryKind = 2
	JournalEntryKindRefreshSuccess        JournalEntryKind = 3
	JournalEntryKindOutputs               JournalEntryKind = 4
	JournalEntryKindWrite                 JournalEntryKind = 5
	JournalEntryKindSecretsManager        JournalEntryKind = 6
	JournalEntryKindRebuiltBaseState      JournalEntryKind = 7
	JournalEntryKindExtensionParameterize JournalEntryKind = 8
	JournalEntryKindSnippets              JournalEntryKind = 9
	JournalEntryKindStateMigration        JournalEntryKind = 10
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
	case JournalEntryKindExtensionParameterize:
		return "extension-parameterize"
	case JournalEntryKindSnippets:
		return "snippets"
	case JournalEntryKindStateMigration:
		return "state-migration"
	default:
		return "invalid"
	}
}

// JournalBaseStatePatch replaces a retained resource in the journal's base snapshot after a state migration.
// State is the complete, already-rewritten checkpoint state; replay must not reinterpret migration successors.
type JournalBaseStatePatch struct {
	Index int64      `json:"index"`
	State ResourceV3 `json:"state"`
}

// JournalNewStatePatch replaces a resource produced by an earlier operation after a state migration.
// OperationID identifies the successful operation that introduced the resource into the journal's new-state list.
type JournalNewStatePatch struct {
	OperationID int64      `json:"operationID"`
	State       ResourceV3 `json:"state"`
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
	// The complete snippet list associated with this journal entry, if any.
	Snippets []SnippetV1 `json:"snippets,omitempty"`

	// NewSnapshot is the new snapshot that this journal entry is associated with.
	NewSnapshot *DeploymentV3 `json:"newSnapshot,omitempty"`

	// ExtensionRef and Extension carry the (ref, blob) pair produced by an
	// extension parameterize step so replay can rebuild DeploymentV3.Extensions.
	// Only set for JournalEntryKindExtensionParameterize entries.
	ExtensionRef *ExtensionRef `json:"extensionRef,omitempty"`
	Extension    *Extension    `json:"extension,omitempty"`

	// True if serializing any resource state carried by this entry encoded strings containing non-UTF8 bytes.
	// Such strings inside secrets are invisible after encryption, so the fact must be recorded at serialization
	// time for replay to gate rebuilt deployments on the byteString feature.
	RequiresByteString bool `json:"requiresByteString,omitempty"`
	// RemoveOlds holds the indices (in increasing order) of the resources in the base snapshot that a state
	// migration removes. Only set for JournalEntryKindStateMigration entries.
	RemoveOlds []int64 `json:"removeOlds,omitempty"`
	// States holds the resources a state migration splices into the base snapshot, in order. They take the
	// position of the last removed resource. Only set for JournalEntryKindStateMigration entries.
	States []ResourceV3 `json:"states,omitempty"`
	// BaseStatePatches contains complete replacements for retained base resources whose references were rewritten.
	// Indices refer to the base snapshot before RemoveOlds is applied.
	BaseStatePatches []JournalBaseStatePatch `json:"baseStatePatches,omitempty"`
	// NewStatePatches contains complete replacements for resources produced by operations earlier in this update.
	NewStatePatches []JournalNewStatePatch `json:"newStatePatches,omitempty"`
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
	if e.Snippets != nil {
		fmt.Fprintf(&sb, ", snippets(%v)", len(e.Snippets))
	}
	if e.RemoveOlds != nil {
		fmt.Fprintf(&sb, ", removeOlds(%v)", len(e.RemoveOlds))
	}
	if e.States != nil {
		fmt.Fprintf(&sb, ", states(%v)", len(e.States))
	}
	if e.BaseStatePatches != nil {
		fmt.Fprintf(&sb, ", baseStatePatches(%v)", len(e.BaseStatePatches))
	}
	if e.NewStatePatches != nil {
		fmt.Fprintf(&sb, ", newStatePatches(%v)", len(e.NewStatePatches))
	}

	return sb.String()
}

type JournalEntries struct {
	Entries []JournalEntry `json:"entries,omitempty"`
}
