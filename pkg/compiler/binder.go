// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

// Binder annotates an existing parse tree with semantic information.
type Binder interface {
	Visitor

	// Bind takes the parse tree, binds all names inside of it, mutating it in place.
	Bind(doc *diag.Document, stack *ast.Stack)
}

func NewBinder(c Compiler) Binder {
	return &binder{c}
}

type binder struct {
	c Compiler
}

func (b *binder) Diag() diag.Sink {
	return b.c.Diag()
}

func (b *binder) Bind(doc *diag.Document, stack *ast.Stack) {
	glog.Infof("Binding Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Binding Mu Stack %v completed w/ %v warnings and %v errors",
				stack.Name, b.Diag().Warnings(), b.Diag().Errors())
		}()
	}
	b.VisitStack(doc, stack)
}

func (b *binder) VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata) {
}

func (b *binder) VisitStack(doc *diag.Document, stack *ast.Stack) {
}

func (b *binder) VisitParameter(doc *diag.Document, name string, param *ast.Parameter) {
}

func (b *binder) VisitService(doc *diag.Document, name string, public bool, svc *ast.Service) {
}

func (b *binder) VisitDependency(doc *diag.Document, name string, dep *ast.Dependency) {
}
