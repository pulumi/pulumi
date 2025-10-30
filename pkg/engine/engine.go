package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// UpdateInfo handles information common to resource operations (update, preview, destroy, import, refresh).
type UpdateInfo = engine.UpdateInfo

// Context provides cancellation, termination, and eventing options for an engine operation. It also provides
// a way for the engine to persist snapshots, using the `SnapshotManager`.
type Context = engine.Context

