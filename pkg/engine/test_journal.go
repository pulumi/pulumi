package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

type TestJournalEntryKind = engine.TestJournalEntryKind

type TestJournalEntry = engine.TestJournalEntry

type JournalEntries = engine.JournalEntries

type TestJournal = engine.TestJournal

const TestJournalEntryBegin = engine.TestJournalEntryBegin

const TestJournalEntrySuccess = engine.TestJournalEntrySuccess

const TestJournalEntryFailure = engine.TestJournalEntryFailure

const TestJournalEntryOutputs = engine.TestJournalEntryOutputs

// NewTestJournal creates a new TestJournal that is used in tests to record journal entries for
// deployment steps. These journal entries are used to reconstruct the snapshot at the end of
// the test. This is used in lifecycletests to check that the snapshot manager and the testjournal
// produce the same snapshot.
func NewTestJournal() *TestJournal {
	return engine.NewTestJournal()
}

// FilterRefreshDeletes filters out any dependencies and parents from 'resources' that refer to a URN that is
// not present in 'resources', due to refreshes. This is pretty much the same as `rebuildBaseState` in the deployment
// executor (see that function for a lot of details about why this is necessary). The main difference is that
// this function does not mutate the state objects in place instead returning a new state object with the
// appropriate fields filtered out, note that the slice containing the states is mutated.
func FilterRefreshDeletes(resources []*resource.State) {
	engine.FilterRefreshDeletes(resources)
}

