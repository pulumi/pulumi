// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Snapshot is a view of a collection of resources in an environment at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot interface {
	Objects() graph.Graph                 // the raw underlying object graph.
	Resources() graph.Graph               // a graph containing just resources and their dependencies.
	Topsort() []Resource                  // a topologically sorted list of resources (based on dependencies).
	ResourceByID(id ID, t Type) Resource  // looks up a resource by ID and type.
	ResourceByMoniker(m Moniker) Resource // looks up a resource by its moniker.
}

// NewSnapshot takes an object graph and produces a resource snapshot from it.  It understands how to name resources
// based on their position within the graph and how to identify and record dependencies.  This function can fail
// dynamically if the input graph did not satisfy the preconditions for resource graphs (like that it is a DAG).
func NewSnapshot(g graph.Graph) (Snapshot, error) {
	// TODO: create the resource graph.
	tops, err := topsort(g)
	if err != nil {
		return nil, err
	}
	return &snapshot{g: g, tops: tops}, nil
}

type snapshot struct {
	g    graph.Graph // the underlying MuGL object graph.
	res  graph.Graph // the MuGL graph containing just resources.
	tops []Resource  // the topologically sorted linearized list of resources.
}

func (s *snapshot) Objects() graph.Graph   { return s.g }
func (s *snapshot) Resources() graph.Graph { return s.res }
func (s *snapshot) Topsort() []Resource    { return s.tops }

func topsort(g graph.Graph) ([]Resource, error) {
	var resources []Resource

	// TODO: we want this to return a *graph*, not a linearized list, so that we can parallelize.
	// TODO: this should actually operate on the resource graph, once it exists.  This has two advantages: first, there
	//     will be less to sort; second, we won't need an extra pass just to prune out the result.
	// TODO: as soon as we create the resource graph, we need to memoize monikers so that we can do lookups.

	// Sort the graph output so that it's a DAG; if it's got cycles, this can fail.
	sorted, err := graph.Topsort(g)
	if err != nil {
		return resources, err
	}

	// Now walk the list and prune out anything that isn't a resource.
	for _, v := range sorted {
		if IsResourceVertex(v) {
			res := NewResource(g, v)
			resources = append(resources, res)
		}
	}

	return resources, nil
}

func (s *snapshot) ResourceByID(id ID, t Type) Resource {
	contract.Failf("TODO: not yet implemented")
	return nil
}

func (s *snapshot) ResourceByMoniker(m Moniker) Resource {
	contract.Failf("TODO: not yet implemented")
	return nil
}
