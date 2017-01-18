// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/compiler/legacy/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Visitor unifies all visitation patterns under a single interface.
type Visitor interface {
	Phase
	VisitStack(stack *ast.Stack)
	VisitSchemas(parent *ast.Stack, schmas *ast.Schemas)
	VisitSchema(pstack *ast.Stack, parent *ast.Schemas, name tokens.Name, public bool, schema *ast.Schema)
	VisitProperty(parent *ast.Stack, schema *ast.Schema, name string, prop *ast.Property)
	VisitServices(parent *ast.Stack, svcs *ast.Services)
	VisitService(pstack *ast.Stack, parent *ast.Services, name tokens.Name, public bool, svc *ast.Service)
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

var _ Visitor = (*inOrderVisitor)(nil) // compile-time assert that inOrderVisitor implements Visitor.

func (v *inOrderVisitor) Diag() diag.Sink {
	if v.pre != nil {
		return v.pre.Diag()
	}
	if v.post != nil {
		return v.post.Diag()
	}
	return nil
}

func (v *inOrderVisitor) VisitStack(stack *ast.Stack) {
	if v.pre != nil {
		v.pre.VisitStack(stack)
	}

	v.VisitSchemas(stack, &stack.Types)
	for _, name := range ast.StableProperties(stack.Properties) {
		v.VisitProperty(stack, nil, name, stack.Properties[name])
	}
	v.VisitServices(stack, &stack.Services)

	if v.post != nil {
		v.post.VisitStack(stack)
	}
}

func (v *inOrderVisitor) VisitSchemas(parent *ast.Stack, schemas *ast.Schemas) {
	if v.pre != nil {
		v.pre.VisitSchemas(parent, schemas)
	}

	for _, name := range ast.StableSchemas(schemas.Private) {
		v.VisitSchema(parent, schemas, name, false, schemas.Private[name])
	}
	for _, name := range ast.StableSchemas(schemas.Public) {
		v.VisitSchema(parent, schemas, name, true, schemas.Public[name])
	}

	if v.post != nil {
		v.post.VisitSchemas(parent, schemas)
	}
}

func (v *inOrderVisitor) VisitSchema(pstack *ast.Stack, parent *ast.Schemas, name tokens.Name,
	public bool, schema *ast.Schema) {
	if v.pre != nil {
		v.pre.VisitSchema(pstack, parent, name, public, schema)
	}

	for _, name := range ast.StableProperties(schema.Properties) {
		v.VisitProperty(pstack, schema, name, schema.Properties[name])
	}

	if v.post != nil {
		v.post.VisitSchema(pstack, parent, name, public, schema)
	}
}

func (v *inOrderVisitor) VisitProperty(parent *ast.Stack, schema *ast.Schema, name string, prop *ast.Property) {
	if v.pre != nil {
		v.pre.VisitProperty(parent, schema, name, prop)
	}
	if v.post != nil {
		v.post.VisitProperty(parent, schema, name, prop)
	}
}

func (v *inOrderVisitor) VisitServices(parent *ast.Stack, svcs *ast.Services) {
	if v.pre != nil {
		v.pre.VisitServices(parent, svcs)
	}

	for _, name := range ast.StableServices(svcs.Private) {
		v.VisitService(parent, svcs, name, false, svcs.Private[name])
	}
	for _, name := range ast.StableServices(svcs.Public) {
		v.VisitService(parent, svcs, name, true, svcs.Public[name])
	}

	if v.post != nil {
		v.post.VisitServices(parent, svcs)
	}
}

func (v *inOrderVisitor) VisitService(pstack *ast.Stack, parent *ast.Services, name tokens.Name,
	public bool, svc *ast.Service) {
	if v.pre != nil {
		v.pre.VisitService(pstack, parent, name, public, svc)
	}
	if v.post != nil {
		v.post.VisitService(pstack, parent, name, public, svc)
	}
}
