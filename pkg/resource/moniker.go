// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Moniker is a friendly, but unique, name for a resource, most often auto-assigned by the Mu system.  (In theory, we
// could support manually assigned monikers in the future, to help with the "stable moniker" problem outlined below).
// These monikers are used to perform graph diffing and resolution of resource object to the underlying provider ID.
type Moniker string

// NewMoniker creates a unique moniker for the given vertex v inside of the given graph g.
func NewMoniker(g graph.Graph, v graph.Vertex) Moniker {
	// The algorithm for generating a moniker is quite simple at the moment.
	//
	// We begain by walk the in-edges to the vertex v, recursively until we hit a root node in g.  The edge path
	// traversed is then inspected for labels and those labels are concatenated with the vertex object's type name to
	// form a moniker -- or string-based "path" -- from the root of the graph to the vertex within it.
	//
	// Note that it is possible there are multiple paths to the object.  In that case, we pick the shortest one.  If
	// there are still more than one that are of equal length, we pick the first lexicographically ordered one.
	//
	// It's worth pointing out the "stable moniker" problem.  Because the moniker is sensitive to the object's path, any
	// changes in this path will alter the moniker.  This can introduce difficulties in diffing two resource graphs.  As
	// a result, I suspect we will go through many iterations of this algortihm, and will need to provider facilities
	// for developers to "rename" existing resources manually and/or even give resources IDs, monikers, etc. manually.
	contract.Failf("Moniker creation not yet implemented")
	return ""
}
