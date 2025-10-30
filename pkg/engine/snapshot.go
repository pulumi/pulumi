package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// SnapshotManager manages an in-memory resource graph.
type SnapshotManager = engine.SnapshotManager

// SnapshotMutation represents an outstanding mutation that is yet to be completed. When the engine completes
// a mutation, it must call `End` in order to record the successful completion of the mutation.
type SnapshotMutation = engine.SnapshotMutation

