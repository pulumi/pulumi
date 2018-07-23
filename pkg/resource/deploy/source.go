// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"io"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// A Source can generate a new set of resources that the planner will process accordingly.
type Source interface {
	io.Closer

	// Project returns the package name of the Pulumi project we are obtaining resources from.
	Project() tokens.PackageName
	// Info returns a serializable payload that can be used to stamp snapshots for future reconciliation.
	Info() interface{}
	// IsRefresh indicates whether this source returns events source from existing state (true), and hence can simply
	// be assumed to reflect existing state, or whether the events should acted upon (false).
	IsRefresh() bool

	// Iterate begins iterating the source.  Error is non-nil upon failure; otherwise, a valid iterator is returned.
	Iterate(opts Options) (SourceIterator, error)
}

// A SourceIterator enumerates the list of resources that a source has to offer and tracks associated state.
type SourceIterator interface {
	io.Closer

	// Next returns the next event from the source.
	Next() (SourceEvent, error)
}

// SourceEvent is an event associated with the enumeration of a plan.  It is an intent expressed by the source
// program, and it is the responsibility of the engine to make it so.
type SourceEvent interface {
	event()
}

// RegisterResourceEvent is a step that asks the engine to provision a resource.
type RegisterResourceEvent interface {
	SourceEvent
	// Goal returns the goal state for the resource object that was allocated by the program.
	Goal() *resource.Goal
	// Done indicates that we are done with this step.  It must be called to perform cleanup associated with the step.
	Done(result *RegisterResult)
}

// RegisterResult is the state of the resource after it has been registered.
type RegisterResult struct {
	State   *resource.State        // the resource state.
	Stable  bool                   // if true, the resource state is stable and may be trusted.
	Stables []resource.PropertyKey // an optional list of specific resource properties that are stable.
}

// RegisterResourceOutputsEvent is an event that asks the engine to complete the provisioning of a resource.
type RegisterResourceOutputsEvent interface {
	SourceEvent
	// URN is the resource URN that this completion applies to.
	URN() resource.URN
	// Outputs returns a property map of output properties to add to a resource before completing.
	Outputs() resource.PropertyMap
	// Done indicates that we are done with this step.  It must be called to perform cleanup associated with the step.
	Done()
}

// ReadResourceEvent is an event that asks the engine to read the state of a resource that already exists.
type ReadResourceEvent interface {
	SourceEvent

	ID() resource.ID
	Name() tokens.QName
	Type() tokens.Type
	Parent() resource.URN
	Properties() resource.PropertyMap
	Dependencies() []resource.URN

	// Done indicates that we are done with this event.
	Done(result *ReadResult)
}

type ReadResult struct {
	State *resource.State
}
