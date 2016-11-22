// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

// Visitor unifies all visitation patterns under a single interface.
type Visitor interface {
	Phase
	VisitStack(doc *diag.Document, stack *ast.Stack)
	VisitCluster(doc *diag.Document, name string, cluster *ast.Cluster)
	VisitProperty(doc *diag.Document, name string, prop *ast.Property)
	VisitDependency(doc *diag.Document, ref ast.Ref, dep *ast.Dependency)
	VisitBoundDependency(doc *diag.Document, ref ast.Ref, dep *ast.BoundDependency)
	VisitServices(doc *diag.Document, svcs *ast.Services)
	VisitService(doc *diag.Document, name ast.Name, public bool, svc *ast.Service)
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

func (v *inOrderVisitor) Diag() diag.Sink {
	if v.pre != nil {
		return v.pre.Diag()
	}
	if v.post != nil {
		return v.post.Diag()
	}
	return nil
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
		v.VisitProperty(doc, name, &prop)
		// Copy the property back in case it was updated.
		stack.Properties[name] = prop
	}

	for _, ref := range ast.StableDependencies(stack.Dependencies) {
		dep := stack.Dependencies[ref]
		v.VisitDependency(doc, ref, &dep)
		// Copy the dependency back in case it was updated.
		stack.Dependencies[ref] = dep
	}

	for _, ref := range ast.StableBoundDependencies(stack.BoundDependencies) {
		dep := stack.BoundDependencies[ref]
		v.VisitBoundDependency(doc, ref, &dep)
		// Copy the dependency back in case it was updated.
		stack.BoundDependencies[ref] = dep
	}

	v.VisitServices(doc, &stack.Services)

	if v.post != nil {
		v.post.VisitStack(doc, stack)
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

func (v *inOrderVisitor) VisitProperty(doc *diag.Document, name string, prop *ast.Property) {
	if v.pre != nil {
		v.pre.VisitProperty(doc, name, prop)
	}
	if v.post != nil {
		v.post.VisitProperty(doc, name, prop)
	}
}

func (v *inOrderVisitor) VisitDependency(doc *diag.Document, ref ast.Ref, dep *ast.Dependency) {
	if v.pre != nil {
		v.pre.VisitDependency(doc, ref, dep)
	}
	if v.post != nil {
		v.post.VisitDependency(doc, ref, dep)
	}
}

func (v *inOrderVisitor) VisitBoundDependency(doc *diag.Document, ref ast.Ref, dep *ast.BoundDependency) {
	if v.pre != nil {
		v.pre.VisitBoundDependency(doc, ref, dep)
	}
	if v.post != nil {
		v.post.VisitBoundDependency(doc, ref, dep)
	}
}

func (v *inOrderVisitor) VisitServices(doc *diag.Document, svcs *ast.Services) {
	if v.pre != nil {
		v.pre.VisitServices(doc, svcs)
	}

	for _, name := range ast.StableServices(svcs.Public) {
		aname := ast.Name(name)
		public := svcs.Public[aname]
		v.VisitService(doc, aname, true, &public)
		// Copy the public service back in case it was updated.
		svcs.Public[aname] = public
	}

	for _, name := range ast.StableServices(svcs.Private) {
		aname := ast.Name(name)
		private := svcs.Private[aname]
		v.VisitService(doc, aname, false, &private)
		// Copy the private service back in case it was updated.
		svcs.Private[aname] = private
	}

	if v.post != nil {
		v.post.VisitServices(doc, svcs)
	}
}

func (v *inOrderVisitor) VisitService(doc *diag.Document, name ast.Name, public bool, svc *ast.Service) {
	if v.pre != nil {
		v.pre.VisitService(doc, name, public, svc)
	}
	if v.post != nil {
		v.post.VisitService(doc, name, public, svc)
	}
}
