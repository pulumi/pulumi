// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"context"

	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// UpdateInfo abstracts away information about an apply, preview, or destroy.
type UpdateInfo interface {
	// GetRoot returns the root directory for this update. This defines the scope for any filesystem resources
	// accessed by this update.
	GetRoot() string
	// GetProject returns information about the project associated with this update. This includes information such as
	// the runtime that will be used to execute the Pulumi program and the program's relative working directory.
	GetProject() *workspace.Project
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

// Context provides cancellation, termination, and eventing options for an engine operation.
type Context struct {
	// cancellationContext is a context that when cancelled will cause the engine to drive its current operation to a
	// safe point and then return.
	cancellationContext context.Context

	// terminationContext is a context that when cancelled will cause the engine to immediately return without driving
	// its current operation to a safe point.
	terminationContext context.Context

	// events is a channel over which to deliver engine events.
	events chan<- Event
}

// NewContext creates a new engine operation context from the given cancellation and termination context and event
// sink.
func NewContext(cancellationContext, terminationContext context.Context, events chan<- Event) *Context {
	contract.Require(events != nil, "events")

	// If no termination context was provided, use the background context as the termination context.
	if terminationContext == nil {
		terminationContext = context.Background()
	}

	// If no cancellation context was provided, use the termination context as the cancellation context.
	if cancellationContext == nil {
		cancellationContext = terminationContext
	}

	return &Context{
		cancellationContext: cancellationContext,
		terminationContext:  terminationContext,
		events:              events,
	}
}

// cancellationErr returns a non-nil error if the context has been cancelled or terminated.
func (c *Context) cancellationErr() error {
	if err := c.cancellationContext.Err(); err != nil {
		return err
	}

	return c.terminationContext.Err()
}
