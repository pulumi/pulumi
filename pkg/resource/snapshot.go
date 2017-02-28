// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"github.com/golang/glog"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/eval/heapstate"
	"github.com/pulumi/coconut/pkg/eval/rt"
	"github.com/pulumi/coconut/pkg/graph"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Snapshot is a view of a collection of resources in an environment at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot interface {
	Ctx() *Context                              // fetches the context for this snapshot.
	Namespace() tokens.QName                    // the husk/namespace target being deployed into.
	Pkg() tokens.Package                        // the package from which this snapshot came.
	Args() core.Args                            // the arguments used to compile this package.
	Resources() []Resource                      // a topologically sorted list of resources (based on dependencies).
	ResourceByID(id ID, t tokens.Type) Resource // looks up a resource by ID and type.
	ResourceByMoniker(m Moniker) Resource       // looks up a resource by its moniker.
	ResourceByObject(obj *rt.Object) Resource   // looks up a resource by its object.
}

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
func NewSnapshot(ctx *Context, ns tokens.QName, pkg tokens.Package,
	args core.Args, resources []Resource) Snapshot {
	return &snapshot{ctx, ns, pkg, args, resources}
}

// NewGraphSnapshot takes an object graph and produces a resource snapshot from it.  It understands how to name
// resources based on their position within the graph and how to identify and record dependencies.  This function can
// fail dynamically if the input graph did not satisfy the preconditions for resource graphs (like that it is a DAG).
func NewGraphSnapshot(ctx *Context, ns tokens.QName, pkg tokens.Package,
	args core.Args, heap *heapstate.Heap) (Snapshot, error) {
	// Topologically sort the entire heapstate (in dependency order) and extract just the resource objects.
	resobjs, err := topsort(ctx, heap.G)
	if err != nil {
		return nil, err
	}

	// Next, name all resources, create their monikers and objects, and maps that we will use.  Note that we must do
	// this in DAG order (guaranteed by our topological sort above), so that referenced monikers are available.
	resources, err := createResources(ctx, ns, heap, resobjs)
	if err != nil {
		return nil, err
	}

	return NewSnapshot(ctx, ns, pkg, args, resources), nil
}

type snapshot struct {
	ctx       *Context       // the context shared by all operations in this snapshot.
	ns        tokens.QName   // the namespace target being deployed into.
	pkg       tokens.Package // the package from which this snapshot came.
	args      core.Args      // the arguments used to compile this package.
	resources []Resource     // the topologically sorted linearized list of resources.
}

func (s *snapshot) Ctx() *Context           { return s.ctx }
func (s *snapshot) Namespace() tokens.QName { return s.ns }
func (s *snapshot) Pkg() tokens.Package     { return s.pkg }
func (s *snapshot) Args() core.Args         { return s.args }
func (s *snapshot) Resources() []Resource   { return s.resources }

func (s *snapshot) ResourceByID(id ID, t tokens.Type) Resource {
	contract.Failf("TODO: not yet implemented")
	return nil
}

func (s *snapshot) ResourceByMoniker(m Moniker) Resource     { return s.ctx.MksRes[m] }
func (s *snapshot) ResourceByObject(obj *rt.Object) Resource { return s.ctx.ObjRes[obj] }

// createResources uses a graph to create monikers and resource objects for every resource within.  It
// returns two maps for further use: a map of vertex to its new resource object, and a map of vertex to its moniker.
func createResources(ctx *Context, husk tokens.QName, heap *heapstate.Heap, resobjs []*rt.Object) ([]Resource, error) {
	var resources []Resource
	for _, resobj := range resobjs {
		// Create an object resource without a moniker.
		res := NewObjectResource(ctx, resobj)

		// Now fetch this resource's name by looking up its provider and doing an RPC.
		t := resobj.Type().TypeToken()
		prov, err := ctx.Provider(t.Package())
		if err != nil {
			return nil, err
		}
		name, err := prov.Name(t, res.Properties())
		if err != nil {
			return nil, err
		}

		// Now compute a unique moniker for this object and ensure we haven't had any collisions.
		alloc := heap.Alloc(resobj)
		moniker := NewMoniker(husk, alloc.Mod.Tok, t, name)
		glog.V(7).Infof("Resource moniker computed: %v", moniker)
		if _, exists := ctx.MksRes[moniker]; exists {
			// If this moniker is already in use, issue an error, ignore this one, and break.  The break is necessary
			// because subsequent resources might contain references to this moniker and would fail to find it.
			ctx.Diag.Errorf(errors.ErrorDuplicateMonikerNames.At(alloc.Loc), moniker)
			break
		} else {
			res.SetMoniker(moniker)
			ctx.ObjRes[resobj] = res
			ctx.MksRes[moniker] = res
			ctx.ObjMks[resobj] = moniker
		}
		resources = append(resources, res)
	}
	return resources, nil
}

// topsort actually performs a topological sort on a resource graph.
func topsort(ctx *Context, g graph.Graph) ([]*rt.Object, error) {
	// Sort the graph output so that it's a DAG; if it's got cycles, this can fail.
	// TODO: we want this to return a *graph*, not a linearized list, so that we can parallelize.
	// TODO: it'd be nice to prune the graph to just the resource objects first, so we don't waste effort.
	sorted, err := graph.Topsort(g)
	if err != nil {
		return nil, err
	}

	// Now walk the list and prune out anything that isn't a resource.
	var resobjs []*rt.Object
	for _, v := range sorted {
		ov := v.(*heapstate.ObjectVertex)
		if IsResourceVertex(ov) {
			resobjs = append(resobjs, ov.Obj())
		}
	}
	return resobjs, nil
}
