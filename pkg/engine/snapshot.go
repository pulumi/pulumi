// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"io"

	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// SnapshotManager is responsible for maintaining the in-memory representation
// of the current state of the resource world. It wraps a mutable data structure
// and, as such, is responsible for serializing access to it.
//
// Fundamentally, the Pulumi engine operates on two snapshots:
//   1. The snapshot used to *create the plan* is considered to be the "Old"
//      snapshot. Resources that exist in this snapshot are considered to exist
//      for the purposes of plan generation. Any modifications to input properties
//      on resources in this snapshot will cause the planner to generate Update steps,
//      while the creation of resources not in this snapshot will cause the planner to
//      generate Create steps.
//   2. The snapshot modified incrementally as the plan progresses. This snapshot is
//      the snapshot contained within a SnapshotManager. As the planner generates and
//      executes steps, it will inform the SnapshotManager of its intention to modify
//      the snapshot. The SnapshotManager will then record this intent to mutate and serialize
//      it to persistent storage so that the snapshot is still valid in the case of catastrophic
//      engine failure.
//
// The engine informs a SnapshotManager of its intent to mutate the snapshot using this protocol:
//   1. The engine calls `BeginMutation` on a SnapshotManager and passes the `Step` that it
//      intends to execute. The SnapshotManager will inspect this step and mark the resources
//      that will be mutated. It will then persist the modified snapshot and return, giving
//      the engine the green light to continue. The SnapshotManager will return a `SnapshotMutation`.
//   2. The engine applies its Step.
//   3. The engine calls `End` on the `SnapshotMutation` returned from `BeginMutation`. The SnapshotManager
//      will then record the mutation to be complete and finalize resource states that changed.
//
// The engine also informs of a SnapshotManager of its intention to register resource outputs, through
// the `RegisterResourceOutputs` call.
//
// The SnapshotManager ensures that all writes to the in-memory snapshot are serialized.
type SnapshotManager interface {
	io.Closer

	// BeginMutation signals to the SnapshotManager that the planner intends to mutate the global
	// snapshot. It provides the step that it intends to execute. Based on that step, BeginMutation
	// will record this intent in the global snapshot and return a `SnapshotMutation` that, when ended,
	// will complete the transaction.
	BeginMutation(step deploy.Step) (SnapshotMutation, error)

	// RecordPlugins records the given list of plugins in the manifest, so that Destroy operations
	// operating on this snapshot know which plugins need to be loaded without having to inspect
	// the program.
	RecordPlugins(plugins []workspace.PluginInfo) error

	// RegisterResourceOutputs registers the outputs of a step with the snapshot, once a step has completed. The
	// engine has populated the `New` resource of the given step with the Outputs that it would like to save in
	// the snapshot.
	RegisterResourceOutputs(step deploy.Step) error
}

// SnapshotMutation represents an outstanding mutation that is yet to be completed. When the engine completes
// a mutation, it must call `End` in order to record the successful completion of the mutation.
type SnapshotMutation interface {
	// End terminates the transaction and commits the results to the snapshot, returning an error if this
	// failed to complete.
	End(step deploy.Step) error
}
