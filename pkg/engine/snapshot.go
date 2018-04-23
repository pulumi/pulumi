package engine

import (
	"io"

	"github.com/pulumi/pulumi/pkg/resource/deploy"
)

// SnapshotManager is responsible for maintaining the in-memory representation
// of the current state of the resource world.
type SnapshotManager interface {
	io.Closer

	// BeginMutation signals to the SnapshotManager that the planner intends to mutate the global
	// snapshot. It provides the step that it intends to execute. Based on that step, BeginMutation
	// will record this intent in the global snapshot and return a `SnapshotMutation` that, when ended,
	// will complete the transaction.
	BeginMutation() (SnapshotMutation, error)
}

// SnapshotMutation represents an outstanding mutation that is yet to be completed. When the engine completes
// a mutation, it must call `End` in order to record the successful completion of the mutation.
type SnapshotMutation interface {
	// End terminates the transaction and commits the results to the snapshot, returning an error if this
	// failed to complete.
	End(snapshot *deploy.Snapshot) error
}
