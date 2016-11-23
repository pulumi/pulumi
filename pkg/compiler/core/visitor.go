// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

// Visitor unifies all visitation patterns under a single interface.
type Visitor interface {
	Phase
	VisitWorkspace(doc *diag.Document, w *ast.Workspace)
	VisitCluster(doc *diag.Document, name string, cluster *ast.Cluster)
	VisitDependency(doc *diag.Document, parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency)
	VisitStack(doc *diag.Document, stack *ast.Stack)
	VisitProperty(doc *diag.Document, parent *ast.Stack, name string, prop *ast.Property)
	VisitBoundDependency(doc *diag.Document, parent *ast.Stack, dep *ast.BoundDependency)
	VisitServices(doc *diag.Document, parent *ast.Stack, svcs *ast.Services)
	VisitService(doc *diag.Document, parent *ast.Services, name ast.Name, public bool, svc *ast.Service)
}

// NewInOrderVisitor wraps another Visitor and walks the tree in a deterministic order, deferring to another set of
// Visitor objects for pre- and/or post-actions.  Either pre or post may be nil.
func NewInOrderVisitor(pre Visitor, post Visitor) Visitor {
	return &inOrderVisitor{pre, post}
}

// inOrderVisitor simply implements the Visitor pattern as specified above.
//
// Note that we need to iterate all maps in a stable order (since Go's are unordered by default).  Sadly, this
// is rather verbose due to Go's lack of generics, reflectionless Keys() functions, and so on.
type inOrderVisitor struct {
	pre  Visitor
	post Visitor
}

var _ Visitor = &inOrderVisitor{} // compile-time assert that inOrderVisitor implements Visitor.

func (v *inOrderVisitor) Diag() diag.Sink {
	if v.pre != nil {
		return v.pre.Diag()
	}
	if v.post != nil {
		return v.post.Diag()
	}
	return nil
}

func (v *inOrderVisitor) VisitWorkspace(doc *diag.Document, w *ast.Workspace) {
	if v.pre != nil {
		v.pre.VisitWorkspace(doc, w)
	}

	for _, name := range ast.StableClusters(w.Clusters) {
		cluster := w.Clusters[name]
		v.VisitCluster(doc, name, &cluster)
		// Copy the cluster back in case it was updated.
		w.Clusters[name] = cluster
	}

	for _, ref := range ast.StableDependencies(w.Dependencies) {
		dep := w.Dependencies[ref]
		v.VisitDependency(doc, w, ref, &dep)
		// Copy the dependency back in case it was updated.
		w.Dependencies[ref] = dep
	}

	if v.post != nil {
		v.post.VisitWorkspace(doc, w)
	}
}

func (v *inOrderVisitor) VisitCluster(doc *diag.Document, name string, cluster *ast.Cluster) {
	if v.pre != nil {
		v.pre.VisitCluster(doc, name, cluster)
	}
	if v.post != nil {
		v.post.VisitCluster(doc, name, cluster)
	}
}

func (v *inOrderVisitor) VisitDependency(doc *diag.Document, parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
	if v.pre != nil {
		v.pre.VisitDependency(doc, parent, ref, dep)
	}
	if v.post != nil {
		v.post.VisitDependency(doc, parent, ref, dep)
	}
}

func (v *inOrderVisitor) VisitStack(doc *diag.Document, stack *ast.Stack) {
	if v.pre != nil {
		v.pre.VisitStack(doc, stack)
	}

	for _, name := range ast.StableClusters(stack.Clusters) {
		cluster := stack.Clusters[name]
		v.VisitCluster(doc, name, &cluster)
		// Copy the cluster back in case it was updated.
		stack.Clusters[name] = cluster
	}

	for _, name := range ast.StableProperties(stack.Properties) {
		prop := stack.Properties[name]
		v.VisitProperty(doc, stack, name, &prop)
		// Copy the property back in case it was updated.
		stack.Properties[name] = prop
	}

	for i := range stack.BoundDependencies {
		v.VisitBoundDependency(doc, stack, &stack.BoundDependencies[i])
	}

	v.VisitServices(doc, stack, &stack.Services)

	if v.post != nil {
		v.post.VisitStack(doc, stack)
	}
}

func (v *inOrderVisitor) VisitProperty(doc *diag.Document, parent *ast.Stack, name string, prop *ast.Property) {
	if v.pre != nil {
		v.pre.VisitProperty(doc, parent, name, prop)
	}
	if v.post != nil {
		v.post.VisitProperty(doc, parent, name, prop)
	}
}

func (v *inOrderVisitor) VisitBoundDependency(doc *diag.Document, parent *ast.Stack, dep *ast.BoundDependency) {
	if v.pre != nil {
		v.pre.VisitBoundDependency(doc, parent, dep)
	}
	if v.post != nil {
		v.post.VisitBoundDependency(doc, parent, dep)
	}
}

func (v *inOrderVisitor) VisitServices(doc *diag.Document, parent *ast.Stack, svcs *ast.Services) {
	if v.pre != nil {
		v.pre.VisitServices(doc, parent, svcs)
	}

	for _, name := range ast.StableServices(svcs.Private) {
		private := svcs.Private[name]
		v.VisitService(doc, svcs, name, false, &private)
		// Copy the private service back in case it was updated.
		svcs.Private[name] = private
	}

	for _, name := range ast.StableServices(svcs.Public) {
		public := svcs.Public[name]
		v.VisitService(doc, svcs, name, true, &public)
		// Copy the public service back in case it was updated.
		svcs.Public[name] = public
	}

	if v.post != nil {
		v.post.VisitServices(doc, parent, svcs)
	}
}

func (v *inOrderVisitor) VisitService(doc *diag.Document, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
	if v.pre != nil {
		v.pre.VisitService(doc, parent, name, public, svc)
	}
	if v.post != nil {
		v.post.VisitService(doc, parent, name, public, svc)
	}
}
