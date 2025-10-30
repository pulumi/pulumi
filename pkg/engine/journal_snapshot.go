package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// Journal defines an interface for journal operations. The underlying implementation of this interface
// is responsible for recording and storing the operations, and constructing a snapshot/storing them
// for later replaying.
type Journal = engine.Journal

// SnapshotPersister is an interface implemented by our backends that implements snapshot
// persistence. In order to fit into our current model, snapshot persisters have two functions:
// saving snapshots and invalidating already-persisted snapshots.
type SnapshotPersister = engine.SnapshotPersister

// JournalSnapshotManager is an implementation of engine.JournalSnapshotManager that inspects steps and performs
// mutations on the global snapshot object serially. This implementation maintains two bits of state: the "base"
// snapshot, which is immutable and represents the state of the world prior to the application
// of the current plan, and a journal of operations, which consists of the operations that are being
// applied on top of the immutable snapshot.
type JournalSnapshotManager = engine.JournalSnapshotManager

type JournalEntryKind = engine.JournalEntryKind

type JournalEntry = engine.JournalEntry

const JournalEntryBegin = engine.JournalEntryBegin

const JournalEntrySuccess = engine.JournalEntrySuccess

const JournalEntryFailure = engine.JournalEntryFailure

const JournalEntryRefreshSuccess = engine.JournalEntryRefreshSuccess

const JournalEntryOutputs = engine.JournalEntryOutputs

const JournalEntryWrite = engine.JournalEntryWrite

const JournalEntrySecretsManager = engine.JournalEntrySecretsManager

const JournalEntryRebuiltBaseState = engine.JournalEntryRebuiltBaseState

// NewJournalSnapshotManager creates a new SnapshotManager for the given stack name, using the
// given persister, default secrets manager and base snapshot.
// 
// It is *very important* that the baseSnap pointer refers to the same Snapshot given to the engine! The engine will
// mutate this object, and the snapshot manager will do pointer comparisons to determine indices
// for journal entries.
func NewJournalSnapshotManager(journal Journal, baseSnap *deploy.Snapshot, sm secrets.Manager) (*JournalSnapshotManager, error) {
	return engine.NewJournalSnapshotManager(journal, baseSnap, sm)
}

