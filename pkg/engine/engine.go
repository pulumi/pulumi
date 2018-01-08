// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
)

// Update abstracts away information about an apply, preview, or destroy.
type Update interface {
	// GetRoot returns the root directory for this update. This defines the scope for any filesystem resources
	// accessed by this update.
	GetRoot() string
	// GetPackage returns information about the package associated with this update. This includes information such as
	// the runtime that will be used to execute the Pulumi program and the program's relative working directory.
	GetPackage() *pack.Package
	// GetTarget returns information about the target of this update. This includes the name of the stack being
	// updated, the configuration values associated with the target and the target's latest snapshot.
	GetTarget() *deploy.Target

	// BeginMutation and SnapshotMutation.End allow a snapshot provider to be robust in the face of failures that occur
	// between the points at which they are called. The semantics are as follows:
	//     1. The engine calls `SnapshotProvider.Begin` to indicate that it is about to mutate the state of the
	//        resources tracked by the snapshot.
	//     2. The engine mutates the state of the resoures tracked by the snapshot.
	//     3. The engine calls `SnapshotMutation.End` with the new snapshot to indicate that the mutation(s) it was
	//        performing has/have finished.
	// During (1), the snapshot provider should record the fact that any currently persisted snapshot is being mutated
	// and cannot be assumed to represent the actual state of the system. This ensures that if the engine crashes during
	// (2), then the current snapshot is known to be unreliable. During (3), the snapshot provider should persist the
	// provided snapshot and record that it is known to be reliable.
	BeginMutation() (SnapshotMutation, error)
}

// SnapshotMutation abstracts away managing changes to snapshots.
type SnapshotMutation interface {
	// End indicates that the current mutation has completed and that its results (given by snapshot) should be
	// persisted. See the comments on SnapshotProvider.BeginMutation for more details.
	End(snapshot *deploy.Snapshot) error
}
