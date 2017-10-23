// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
)

type Engine struct {
	Targets   TargetProvider
	Snapshots SnapshotProvider
}

// TargetProvider abstracts away retriving a target
type TargetProvider interface {
	GetTarget(name tokens.QName) (*deploy.Target, error)
}

// SnapshotMutation abstracts away managing changes to snapshots
type SnapshotMutation interface {
	// End indicates that the current mutation has completed and that its results (given by snapshot) should be persisted. See the comments
	// on SnapshotProvider.BeginMutation for more details.
	End(snapshot *deploy.Snapshot) error
}

// SnapshotProvider abstracts away retrieving and storing snapshots
type SnapshotProvider interface {
	GetSnapshot(name tokens.QName) (*deploy.Snapshot, error)

	// BeginMutation and SnapshotMutation.End allow a snapshot provider to be robust in the face of failures that occur between the points
	// at which they are called. The semantics are as follows:
	//     1. The engine calls `SnapshotProvider.Begin` to indicate that it is about to mutate the state of the resources tracked by the
	//        snapshot.
	//     2. The engine mutates the state of the resoures tracked by the snapshot.
	//     3. The engine calls `SnapshotMutation.End` with the new snapshot to indicate that the mutation(s) it was performing has/have
	//        finished.
	// During (1), the snapshot provider should record the fact that any currently persisted snapshot is being mutated and cannot be
	// assumed to represent the actual state of the system. This ensures that if the engine crashes during (2), then the current snapshot
	// is known to be unreliable. During (3), the snapshot provider should persist the provided snapshot and record that it is known to be
	// reliable.
	BeginMutation(name tokens.QName) (SnapshotMutation, error)
}
