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

// A SourceIterator enumerates the list of resources that a source has to offer.
type SourceIterator interface {
	io.Closer
	// Next returns the next resource from the source.  This object contains information produced by the iterator
	// about a resource's state; it may be used to communicate the result of the ensuing planning or deployment
	// operation.  Indeed, its Done function *must* be callled when done.  If it is nil, then the iterator has
	// completed its job and no subsequent calls to Next should be made.
	Next() (SourceGoal, error)
}

// SourceGoal is an item returned from a source iterator which can be used to inspect input state, and
// communicate back the final results after a plan or deployment operation has been performed.
type SourceGoal interface {
	// Resource reflects the goal state for the resource object that was allocated by the program.
	Resource() *resource.Goal
	// Done indicates that we are done with this resource, and provides the full state (ID, URN, and output properties)
	// that resulted from the operation.  This *must* be called when the resource element is done with.
	Done(state *resource.State, stable bool, stables []resource.PropertyKey)
}
