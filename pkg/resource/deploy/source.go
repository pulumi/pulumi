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
	"context"
	"io"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

// A ProviderSource allows a Source to lookup provider plugins.
type ProviderSource interface {
	// GetProvider fetches the provider plugin for the given reference.
	GetProvider(ref providers.Reference) (plugin.Provider, bool)
}

// A Source can generate a new set of resources that the planner will process accordingly.
type Source interface {
	io.Closer

	// Project returns the package name of the Pulumi project we are obtaining resources from.
	Project() tokens.PackageName
	// Info returns a serializable payload that can be used to stamp snapshots for future reconciliation.
	Info() interface{}

	// Iterate begins iterating the source. Error is non-nil upon failure; otherwise, a valid iterator is returned.
	Iterate(ctx context.Context, opts Options, providers ProviderSource) (SourceIterator, result.Result)
}

// A SourceIterator enumerates the list of resources that a source has to offer and tracks associated state.
type SourceIterator interface {
	io.Closer

	// Next returns the next event from the source.
	Next() (SourceEvent, result.Result)
}

// SourceResourceMonitor directs resource operations from the `Source` to various resource
// providers.
type SourceResourceMonitor interface {
	// NOTE: This interface does not implement pulumirpc.ResourceMonitorClient because the eval and
	// query implementations of `Source` do not implement precisely the same signatures.

	Address() string
	Cancel() error
	Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error)
	ReadResource(ctx context.Context,
		req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error)
	RegisterResource(ctx context.Context,
		req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)
	RegisterResourceOutputs(ctx context.Context,
		req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error)
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
	State *resource.State // the resource state.
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

	// ID is the requested ID of this read.
	ID() resource.ID
	// Name is the requested name of this read.
	Name() tokens.QName
	// Type is type of the resource being read.
	Type() tokens.Type
	// Provider is a reference to the provider instance to use for this read.
	Provider() string
	// Parent is the parent resource of the resource being read.
	Parent() resource.URN
	// Properties is the property bag that will be passed to Read as search parameters.
	Properties() resource.PropertyMap
	// Dependencies returns the list of URNs upon which this read depends.
	Dependencies() []resource.URN
	// Done indicates that we are done with this event.
	Done(result *ReadResult)
	// The names of any additional outputs that should be treated as secrets.
	AdditionalSecretOutputs() []resource.PropertyKey
}

type ReadResult struct {
	State *resource.State
}
