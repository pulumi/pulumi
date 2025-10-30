package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// CombinedManager combines multiple SnapshotManagers into one, it simply forwards on each call to every manager.
type CombinedManager = engine.CombinedManager

type CombinedMutation = engine.CombinedMutation

