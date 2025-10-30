package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

type JournalReplayer = backend.JournalReplayer

type SnapshotJournaler = backend.SnapshotJournaler

// A JournalPersister implements persistence of journal entries in some store.
type JournalPersister = backend.JournalPersister

func SerializeJournalEntry(ctx context.Context, je engine.JournalEntry, enc config.Encrypter) (apitype.JournalEntry, error) {
	return backend.SerializeJournalEntry(ctx, je, enc)
}

func NewJournalReplayer(base *apitype.DeploymentV3) *JournalReplayer {
	return backend.NewJournalReplayer(base)
}

// NewSnapshotJournaler creates a new Journal that uses a SnapshotPersister to persist the
// snapshot created from the journal entries.
// 
// The snapshot code works on journal entries. Each resource step produces new journal entries
// for beginning and finishing an operation. These journal entries can then be replayed
// in conjunction with the immutable base snapshot, to rebuild the new snapshot.
// 
// Currently the backend only supports saving full snapshots, in which case only one journal
// entry is allowed to be processed at a time. In the future journal entries will be processed
// asynchronously in the cloud backend, allowing for better throughput for independent operations.
// 
// Serialization is performed by pushing the journal entries onto a channel, where another
// goroutine is polling the channel and creating new snapshots using the entries as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
// 
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` field. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
func NewSnapshotJournaler(ctx context.Context, persister SnapshotPersister, secretsManager secrets.Manager, secretsProvider secrets.Provider, baseSnap *deploy.Snapshot) (*SnapshotJournaler, error) {
	return backend.NewSnapshotJournaler(ctx, persister, secretsManager, secretsProvider, baseSnap)
}

// NewSnapshotJournaler creates a new Journal that uses a SnapshotPersister to persist the
// snapshot created from the journal entries.
// 
// The snapshot code works on journal entries. Each resource step produces new journal entries
// for beginning and finishing an operation. These journal entries can then be replayed
// in conjunction with the immutable base snapshot, to rebuild the new snapshot.
// 
// Currently the backend only supports saving full snapshots, in which case only one journal
// entry is allowed to be processed at a time. In the future journal entries will be processed
// asynchronously in the cloud backend, allowing for better throughput for independent operations.
// 
// Serialization is performed by pushing the journal entries onto a channel, where another
// goroutine is polling the channel and creating new snapshots using the entries as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
// 
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` field. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
func NewJournaler(ctx context.Context, persister JournalPersister, secretsManager secrets.Manager, baseSnap *deploy.Snapshot) (engine.Journal, error) {
	return backend.NewJournaler(ctx, persister, secretsManager, baseSnap)
}

