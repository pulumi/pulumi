// Copyright 2016 Marapongo, Inc. All rights reserved.

// Package heapstate turns MuIL object creation, assignment, etc. events into a MuGL object graph.
package heapstate

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/eval"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Generator listens for events, records them as graph vertices and edges, and returns a DAG afterwards.
type Generator interface {
	eval.InterpreterHooks
	Graph() *ObjectGraph
}

// New allocates a fresh generator, ready to produce graphs.
func New(ctx *core.Context) Generator {
	return &generator{
		ctx:     ctx,
		objs:    make(dependsSet),
		globals: make(objectSet),
	}
}

type generator struct {
	ctx     *core.Context // the compiler context shared between passes.
	objs    dependsSet    // a full set of objects and their dependencies.
	globals objectSet     // a global set of objects (TODO: make this stack-aware).
}

// objectSet is a set of object pointers; each entry has a ref-count to track how many occurrences it contains.
type objectSet map[*rt.Object]int

// dependsSet is a map of object pointers to the objectSet containing the set of objects each such object depends upon.
type dependsSet map[*rt.Object]objectSet

var _ Generator = (*generator)(nil)

// Graph takes the information recorded thus far and produces a new MuGL graph from it.
func (g *generator) Graph() *ObjectGraph {
	glog.V(7).Infof("Generating graph with %v vertices", len(g.objs))

	// First create vertices for all objects.
	verts := make(map[*rt.Object]*ObjectVertex)
	for o := range g.objs {
		verts[o] = NewObjectVertex(o)
	}

	// Now create edges connecting all vertices along dependency lines.
	edges := int64(0)
	for o, targets := range g.objs {
		for target := range targets {
			verts[o].ConnectTo(verts[target])
			edges++
		}
	}
	glog.V(7).Infof("Generated graph with %v edges", edges)

	// Finally, find all vertices that do not have any incoming edges, and consider them roots.
	var roots []*ObjectEdge
	for _, vert := range verts {
		if len(vert.Ins()) == 0 {
			e := NewObjectEdge(nil, vert)
			roots = append(roots, e)
		}
	}
	glog.V(7).Infof("Generated graph with %v roots", len(roots))

	return NewObjectGraph(roots)
}

// OnNewObject is called whenever a new object has been allocated.
func (g *generator) OnNewObject(o *rt.Object) {
	contract.Assert(o != nil)
	glog.V(9).Infof("GraphGenerator OnNewObject t=%v, o=%v", o.Type(), o)

	// Add an entry to the depends set.  It could already exist if it's one of the few "special" object types -- like
	// boolean and null -- that have a fixed number of constant objects.
	// TODO: eventually we may want to be smarter about this, since tracking all dependencies will obviously create
	//     space leaks.  For instance, we could try to narrow down the objects we track to just those rooted by
	//     resources -- since ultimately that's all we will care about -- however, doing that requires (expensive)
	//     reachability analysis that we obviously wouldn't want to perform on each variable assignment.  Another
	//     option would be to periodically garbage collect the heap, clearing out any objects that aren't rooted by a
	//     resource.  This would amortize the cost of scanning, but clearly would be somewhat complex to implement.
	if _, has := g.objs[o]; !has {
		g.objs[o] = make(objectSet) // dependencies start out empty.
	}
}

// OnVariableAssign is called whenever a property has been (re)assigned; it receives both the old and new values.
func (g *generator) OnVariableAssign(o *rt.Object, name tokens.Name, old *rt.Object, nw *rt.Object) {
	glog.V(9).Infof("GraphGenerator OnVariableAssign %v.%v=%v (old=%v)", o, name, nw, old)

	// Unconditionally track all object dependencies.
	var deps objectSet
	if o == nil {
		deps = g.globals
	} else {
		deps = g.objs[o]
		contract.Assert(deps != nil) // we should have seen this object already
	}
	contract.Assert(deps != nil)

	// If the old object is a resource, drop a ref-count.
	if old != nil && old.Type() != types.Null {
		c, has := deps[old]
		contract.Assertf(has, "Expected old resource property to exist in dependency map")
		contract.Assertf(c > 0, "Expected old resource property ref-count to be > 0 in dependency map")
		deps[old] = c - 1
	}

	// If the new object is non-nil, add a ref-count (or a whole new entry if needed).
	if nw != nil && nw.Type() != types.Null {
		if c, has := deps[nw]; has {
			contract.Assertf(c >= 0, "Expected old resource property ref-count to be >= 0 in dependency map")
			deps[nw] = c + 1
		} else {
			deps[nw] = 1
		}
	}
}

// OnEnterPackage is invoked whenever we enter a new package.
func (g *generator) OnEnterPackage(pkg *symbols.Package) {
	glog.V(9).Infof("GraphGenerator OnEnterPackage %v", pkg)
}

// OnLeavePackage is invoked whenever we enter a new package.
func (g *generator) OnLeavePackage(pkg *symbols.Package) {
	glog.V(9).Infof("GraphGenerator OnLeavePackage %v", pkg)
}

// OnEnterModule is invoked whenever we enter a new module.
func (g *generator) OnEnterModule(mod *symbols.Module) {
	glog.V(9).Infof("GraphGenerator OnEnterModule %v", mod)
}

// OnLeaveModule is invoked whenever we enter a new module.
func (g *generator) OnLeaveModule(mod *symbols.Module) {
	glog.V(9).Infof("GraphGenerator OnLeaveModule %v", mod)
}

// OnEnterFunction is invoked whenever we enter a new function.
func (g *generator) OnEnterFunction(fnc symbols.Function) {
	glog.V(9).Infof("GraphGenerator OnEnterFunction %v", fnc)
}

// OnLeaveFunction is invoked whenever we enter a new function.
func (g *generator) OnLeaveFunction(fnc symbols.Function) {
	glog.V(9).Infof("GraphGenerator OnLeaveFunction %v", fnc)
}
