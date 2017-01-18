// Copyright 2016 Marapongo, Inc. All rights reserved.

// The graph package defines MuGL graphs.  Each graph is directed and acyclic, and the nodes have been topologically
// sorted based on dependencies (edges) between them.  Each node in the graph has a type and a set of properties.
//
// There are two forms of graph: complete and incomplete.  A complete graph is one in which all nodes and their property
// values are known.  An incomplete graph is one where two uncertainties may arise: (1) an edge might be "conditional",
// indicating that its presence or absence is dependent on a piece of information not yet available (like an output
// property from a resource), and/or (2) a property may either be similarly conditional or computed as an output value.
//
// In general, Mu blueprints may be compiled into graphs.  These graphs may then be compared to other graphs to produce
// and/or carry out deployment plans.  This package therefore also exposes operations necessary for diffing graphs.
package graph

import (
	"github.com/marapongo/mu/pkg/symbols"
)

// Graph is an instance of a MuGL graph.  Each is associated with a single blueprint MuPackage as its input, along with
// a set of optional arguments used to evaluate it, along with the output DAG with node types and properties.
type Graph interface {
	Pkg() symbols.Name   // the blueprint package that this graph corresponds to.
	Args() []interface{} // the parsed arguments used to instantiate this blueprint.
	Root() Node          // the root node in the graph.
}
