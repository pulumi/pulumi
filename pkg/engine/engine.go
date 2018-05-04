// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/cancel"
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
}

// Context provides cancellation, termination, and eventing options for an engine operation. It also provides
// a way for the engine to persist snapshots, using the `SnapshotManager`.
type Context struct {
	Cancel          *cancel.Context
	Events          chan<- Event
	SnapshotManager SnapshotManager
	ParentSpan      opentracing.SpanContext
}
