// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

type Binder interface {
	Pass

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
	b.bindStack(doc, stack)
}

func (b *binder) bindStack(doc *diag.Document, stack *ast.Stack) {
}
