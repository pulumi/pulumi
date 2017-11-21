// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"io"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// A Source can generate a new set of resources that the planner will process accordingly.
type Source interface {
	io.Closer
	// Pkg returns the package name of the Pulumi program we are obtaining resources from.
	Pkg() tokens.PackageName
	// Info returns a serializable payload that can be used to stamp snapshots for future reconciliation.
	Info() interface{}
	// Iterate begins iterating the source.  Error is non-nil upon failure; otherwise, a valid iterator is returned.
	Iterate(opts Options) (SourceIterator, error)
}

// A SourceIterator enumerates the list of resources that a source has to offer and tracks associated state.
type SourceIterator interface {
	io.Closer
	// Next returns the next intent from the source.
	Next() (SourceIntent, error)
}

// SourceIntent is an intent associated with the enumeration of a plan.  It is an intent expressed by the source
// program, and it is the responsibility of the engine to make it so.
type SourceIntent interface {
	intent()
}

// RegisterIntent is a step that asks the engine to provision a resource.
type RegisterIntent interface {
	SourceIntent
	// Goal returns the goal state for the resource object that was allocated by the program.
	Goal() *resource.Goal
	// Done indicates that we are done with this step.  It must be called to perform cleanup associated with the step.
	Done(urn resource.URN)
}

// CompleteIntent is an intent that asks the engine to complete the provisioning of a resource.
type CompleteIntent interface {
	SourceIntent
	// URN is the resource URN that this completion applies to.
	URN() resource.URN
	// Extras returns an optional "extra" property map of output properties to add to a resource before completing.
	Extras() resource.PropertyMap
	// Done indicates that we are done with this step.  It must be called to perform cleanup associated with the step.
	Done(res *FinalState)
}

// FinalState is the final source completion information.
type FinalState struct {
	State   *resource.State        // the resource state.
	Stable  bool                   // if true, the resource state is stable and may be trusted.
	Stables []resource.PropertyKey // an optional list of specific resource properties that are stable.
}
