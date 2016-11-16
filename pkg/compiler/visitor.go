// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"sort"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

// Visitor unifies all visitation patterns under a single interface.
type Visitor interface {
	Phase
	VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata)
	VisitStack(doc *diag.Document, stack *ast.Stack)
	VisitParameter(doc *diag.Document, name string, param *ast.Parameter)
	VisitService(doc *diag.Document, name ast.Name, public bool, svc *ast.Service)
	VisitDependency(doc *diag.Document, name ast.Name, dep *ast.Dependency)
}

// NewInOrderVisitor wraps another Visitor and walks the tree in a deterministic order, deferring to another set of
// Visitor objects for pre- and/or post-actions.  Either pre or post may be nil.
func NewInOrderVisitor(pre Visitor, post Visitor) Visitor {
	return &inOrderVisitor{pre, post}
}

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

func (v *inOrderVisitor) VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata) {
	if v.pre != nil {
		v.pre.VisitMetadata(doc, kind, meta)
	}
	if v.post != nil {
		v.post.VisitMetadata(doc, kind, meta)
	}
}

func (v *inOrderVisitor) VisitStack(doc *diag.Document, stack *ast.Stack) {
	if v.pre != nil {
		v.pre.VisitStack(doc, stack)
	}

	v.VisitMetadata(doc, "Stack", &stack.Metadata)

	// Note that we need to iterate all maps in a stable order (since Go's are unordered by default).  Sadly, this
	// is rather verbose due to Go's lack of generics, reflectionless Keys() functions, and so on.

	params := make([]string, 0, len(stack.Parameters))
	for param := range stack.Parameters {
		params = append(params, param)
	}
	sort.Strings(params)
	for _, name := range params {
		v.VisitParameter(doc, name, stack.Parameters[name])
	}

	publics := make([]string, 0, len(stack.Public))
	for public := range stack.Public {
		publics = append(publics, string(public))
	}
	sort.Strings(publics)
	for _, name := range publics {
		aname := ast.Name(name)
		v.VisitService(doc, aname, true, stack.Public[aname])
	}

	privates := make([]string, 0, len(stack.Private))
	for private := range stack.Private {
		privates = append(privates, string(private))
	}
	sort.Strings(privates)
	for _, name := range privates {
		aname := ast.Name(name)
		v.VisitService(doc, aname, false, stack.Private[aname])
	}

	deps := make([]string, 0, len(stack.Dependencies))
	for dep := range stack.Dependencies {
		deps = append(deps, string(dep))
	}
	sort.Strings(deps)
	for _, name := range deps {
		aname := ast.Name(name)
		dep := stack.Dependencies[aname]
		v.VisitDependency(doc, aname, &dep)
	}

	if v.post != nil {
		v.post.VisitStack(doc, stack)
	}
}

func (v *inOrderVisitor) VisitParameter(doc *diag.Document, name string, param *ast.Parameter) {
	if v.pre != nil {
		v.pre.VisitParameter(doc, name, param)
	}
	if v.post != nil {
		v.post.VisitParameter(doc, name, param)
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

func (v *inOrderVisitor) VisitDependency(doc *diag.Document, name ast.Name, dep *ast.Dependency) {
	if v.pre != nil {
		v.pre.VisitDependency(doc, name, dep)
	}
	if v.post != nil {
		v.post.VisitDependency(doc, name, dep)
	}
}
