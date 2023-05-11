package apitype

type JournalEntryKind int

const (
	JournalEntryInvalid            JournalEntryKind = 0
	JournalEntryBegin              JournalEntryKind = 1
	JournalEntrySuccess            JournalEntryKind = 2
	JournalEntryFailure            JournalEntryKind = 3
	JournalEntryOutputs            JournalEntryKind = 4
	JournalEntryPendingDeletion    JournalEntryKind = 5
	JournalEntryPendingReplacement JournalEntryKind = 6
	JournalEntrySecrets            JournalEntryKind = 7
)

type JournalEntry struct {
	SequenceNumber int                 `json:"sequence"`
	Kind           JournalEntryKind    `json:"kind"`
	Op             OpType              `json:"op"`
	Old            int                 `json:"old,omitempty"`
	New            int                 `json:"new,omitempty"`
	State          *ResourceV3         `json:"state,omitempty"`
	Secrets        *SecretsProvidersV1 `json:"secrets,omitempty"`
}
