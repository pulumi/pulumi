package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// Snapshot is a view of a collection of resources in an stack at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot = deploy.Snapshot

// SnapshotMetadata contains metadata about a snapshot.
type SnapshotMetadata = deploy.SnapshotMetadata

// SnapshotIntegrityErrorMetadata contains metadata about a snapshot integrity error, such as the version
// and invocation of the Pulumi engine that caused it.
type SnapshotIntegrityErrorMetadata = deploy.SnapshotIntegrityErrorMetadata

// A PruneResult describes the changes made to a resource in a snapshot as a result of pruning dangling dependencies.
type PruneResult = deploy.PruneResult

// A snapshot integrity error is raised when a snapshot is found to be malformed
// or invalid in some way (e.g. missing or out-of-order dependencies, or
// unparseable data).
type SnapshotIntegrityError = deploy.SnapshotIntegrityError

// The set of operations alongside which snapshot integrity checks can be
// performed.
type SnapshotIntegrityOperation = deploy.SnapshotIntegrityOperation

const SnapshotIntegrityWrite = deploy.SnapshotIntegrityWrite

const SnapshotIntegrityRead = deploy.SnapshotIntegrityRead

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
// This property is not checked; for verification, please refer to the VerifyIntegrity function below.
func NewSnapshot(manifest Manifest, secretsManager secrets.Manager, resources []*resource.State, ops []resource.Operation, metadata SnapshotMetadata) *Snapshot {
	return deploy.NewSnapshot(manifest, secretsManager, resources, ops, metadata)
}

// Creates a new snapshot integrity error with a message produced by the given
// format string and arguments. Supports wrapping errors with %w. Snapshot
// integrity errors are raised by Snapshot.VerifyIntegrity when a problem is
// detected with a snapshot (e.g. missing or out-of-order dependencies, or
// unparseable data).
func SnapshotIntegrityErrorf(format string, args ...any) error {
	return deploy.SnapshotIntegrityErrorf(format, args...)
}

// Returns a tuple in which the second element is true if and only if any error
// in the given error's tree is a SnapshotIntegrityError. In that case, the
// first element will be the first SnapshotIntegrityError in the tree. In the
// event that there is no such SnapshotIntegrityError, the first element will be
// nil.
func AsSnapshotIntegrityError(err error) (*SnapshotIntegrityError, bool) {
	return deploy.AsSnapshotIntegrityError(err)
}

