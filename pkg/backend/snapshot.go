package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

// SnapshotPersister is an interface implemented by our backends that implements snapshot
// persistence. In order to fit into our current model, snapshot persisters have two functions:
// saving snapshots and invalidating already-persisted snapshots.
type SnapshotPersister = backend.SnapshotPersister

// SnapshotManager is an implementation of engine.SnapshotManager that inspects steps and performs
// mutations on the global snapshot object serially. This implementation maintains two bits of state: the "base"
// snapshot, which is completely immutable and represents the state of the world prior to the application
// of the current plan, and a "new" list of resources, which consists of the resources that were operated upon
// by the current plan.
// 
// Important to note is that, although this SnapshotManager is designed to be easily convertible into a thread-safe
// implementation, the code as it is today is *not thread safe*. In particular, it is not legal for there to be
// more than one `SnapshotMutation` active at any point in time. This is because this SnapshotManager invalidates
// the last persisted snapshot in `BeginSnapshot`. This is designed to match existing behavior and will not
// be the state of things going forward.
// 
// The resources stored in the `resources` slice are pointers to resource objects allocated by the engine.
// This is subtle and a little confusing. The reason for this is that the engine directly mutates resource objects
// that it creates and expects those mutations to be persisted directly to the snapshot.
type SnapshotManager = backend.SnapshotManager

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking = backend.DisableIntegrityChecking

// NewSnapshotManager creates a new SnapshotManager for the given stack name, using the given persister, default secrets
// manager and base snapshot.
// 
// It is *very important* that the baseSnap pointer refers to the same Snapshot given to the engine! The engine will
// mutate this object and correctness of the SnapshotManager depends on being able to observe this mutation. (This is
// not ideal...)
func NewSnapshotManager(persister SnapshotPersister, secretsManager secrets.Manager, baseSnap *deploy.Snapshot) *SnapshotManager {
	return backend.NewSnapshotManager(persister, secretsManager, baseSnap)
}

