// Copyright 2016-2025, Pulumi Corporation.  All rights reserved.

package apitype

type JournalEntryKind int

const (
	JournalEntryBegin   JournalEntryKind = 0
	JournalEntrySuccess JournalEntryKind = 1
	JournalEntryFailure JournalEntryKind = 2
	JournalEntryOutputs JournalEntryKind = 4
)

type JournalEntry struct {
	Kind             JournalEntryKind `json:"kind"`
	OperationID      uint64           `json:"operationID"`         // ID of the operation this journal entry is associated with.
	DeleteOld        int              `json:"deleteOld"`           // ID for the delete Operation that this journal entry is associated with.
	DeleteNew        uint64           `json:"deleteNew"`           // ID for the delete Operation that this journal entry is associated with.
	State            *ResourceV3      `json:"state,omitempty"`     // The resource state associated with this journal entry.
	Operation        *OperationV2     `json:"operation,omitempty"` // The operation associated with this journal entry, if any.
	RefreshDeleteURN string           `json:"refreshDeleteURN"`    // The URN of the resource that was deleted by a refresh operation.
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
