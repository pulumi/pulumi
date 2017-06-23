// Copyright 2016-2017, Pulumi Corporation
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

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
)

// A Source can generate a new set of resources that the planner will process accordingly.
type Source interface {
	io.Closer
	// Info returns a serializable payload that can be used to stamp snapshots for future reconciliation.
	Info() interface{}
	// Iterate begins iterating the source.  Error is non-nil upon failure; otherwise, a valid iterator is returned.
	Iterate() (SourceIterator, error)
}

// A SourceIterator enumerates the list of resources that a source has to offer.
type SourceIterator interface {
	io.Closer
	// Produce registers a resource that was produced during the iteration, to publish next time.
	Produce(res *resource.Object)
	// Next returns the next step from the source.  If the source allocation is non-nil, it represents the creation of
	// a resource object; if query is non-nil, it represents querying the resources; if both error and the other
	// objects are nil, then the iterator has completed its job and no subsequent calls to next should be made.
	Next() (*SourceAllocation, *SourceQuery, error)
}

// SourceAllocation is used when a resource object is allocated.
type SourceAllocation struct {
	Obj *resource.Object // the resource object.
	Ctx tokens.Module    // the context in which the resource was allocated, used in the production of URNs.
}

// SourceQuery is used when a query function is to be performed.
type SourceQuery struct {
	Type        symbols.Type         // the type of resource being queried.
	GetID       resource.ID          // the resource ID to get (for gets only).
	QueryFilter resource.PropertyMap // the query's filter (for queries only).
}
