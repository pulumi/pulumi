// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// Binder annotates an existing parse tree with semantic information.
type Binder interface {
	// Bind takes the parse tree, binds all names inside of it, mutating it in place.
	Bind(doc *diag.Document, stack *ast.Stack)
}

func NewBinder(c Compiler) Binder {
	return &binder{c, make(map[ast.Name]*Symbol)}
}

type binder struct {
	c      Compiler
	symtbl map[ast.Name]*Symbol
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

	// The binding logic is split into two-phases, due to the possibility of intra-stack references between elements.
	phase1 := &binderPhase1{b, doc}
	phase2 := &binderPhase2{b, doc}

	// Now walk the trees.  We use an InOrderVisitor to do this in the right order, handling determinism, etc. for us.
	v1 := NewInOrderVisitor(phase1, nil)
	v1.VisitStack(doc, stack)

	v2 := NewInOrderVisitor(phase2, nil)
	v2.VisitStack(doc, stack)
}

// RegisterSymbol registers a symbol with the given name; if it already exists, the function returns false.
func (b *binder) RegisterSymbol(sym *Symbol) bool {
	nm := sym.Name
	if _, exists := b.symtbl[nm]; exists {
		return false
	}

	b.symtbl[nm] = sym
	return true
}

type binderPhase1 struct {
	b   *binder
	doc *diag.Document
}

func (p *binderPhase1) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase1) VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata) {
}

func (p *binderPhase1) VisitStack(doc *diag.Document, stack *ast.Stack) {
}

func (p *binderPhase1) VisitParameter(doc *diag.Document, name string, param *ast.Parameter) {
}

func (p *binderPhase1) VisitDependency(doc *diag.Document, name ast.Name, dep *ast.Dependency) {
}

func (p *binderPhase1) VisitServices(doc *diag.Document, svcs *ast.Services) {
}

func (p *binderPhase1) VisitService(doc *diag.Document, name ast.Name, public bool, svc *ast.Service) {
	// Each service has a type.  There are two forms of specifying a type, and this phase will normalize this to a
	// single canonical form to simplify subsequent phases.  First, there is a shorthand form:
	//
	//		private:
	//			acmecorp/db:
	//				...
	//
	// In this example, "acmecorp/db" is the type and the name is shortened to just "db".  Second, there is a longhand
	// form for people who want more control over the naming of their services:
	//
	//		private:
	//			customers:
	//				type: acmecorp/db
	//				...
	//
	// In this example, "acmecorp/db" is still the type, however the name is given the nicer name of "customers."
	if svc.Type == "" {
		svc.Type = svc.Name
		svc.Name = ast.NamePart(svc.Name)
	}

	// Next, note that service definitions can "refer" to other service definitions within the same file.  Any
	// unqualified name is interpreted as such.  As a result, we must add this service to the symbol table even before
	// doing any subsequent binding of its type, etc.  This simplifies the 2nd phase of binding which can rely on this
	// fact, making its logic far simpler.
	sym := NewServiceSymbol(svc.Name, svc)
	if !p.b.RegisterSymbol(sym) {
		p.Diag().Errorf(errors.SymbolAlreadyExists.WithDocument(p.doc), sym.Name)
	}
}

type binderPhase2 struct {
	b   *binder
	doc *diag.Document
}

func (p *binderPhase2) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase2) VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata) {
}

func (p *binderPhase2) VisitStack(doc *diag.Document, stack *ast.Stack) {
}

func (p *binderPhase2) VisitParameter(doc *diag.Document, name string, param *ast.Parameter) {
}

func (p *binderPhase2) VisitDependency(doc *diag.Document, name ast.Name, dep *ast.Dependency) {
}

func (p *binderPhase2) VisitServices(doc *diag.Document, svcs *ast.Services) {
}

func (p *binderPhase2) VisitService(doc *diag.Document, name ast.Name, public bool, svc *ast.Service) {
	// The service's type has been prepared in phase 1, and must now be bound to a symbol.  All shorthand type
	// expressions, intra stack references, cycles, and so forth, will have been taken care of by this earlier phase.
}
