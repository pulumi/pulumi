// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/predef"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
)

// Binder annotates an existing parse tree with semantic information.
type Binder interface {
	core.Phase

	// Bind takes the parse tree, binds all names inside of it, mutating it in place.
	Bind(doc *diag.Document, stack *ast.Stack)
}

func NewBinder(c Compiler) Binder {
	// Create a new binder and a new scope with an empty symbol table.
	b := &binder{c: c}
	b.PushScope()

	// And now populate that symbol table with all known predefined Stack types before returning it.
	for nm, stack := range predef.Stacks {
		sym := NewStackSymbol(nm, stack)
		ok := b.RegisterSymbol(sym)
		util.AssertMF(ok, "Unexpected Symbol collision when registering predef Stack type %v", nm)
	}

	return b
}

type binder struct {
	c     Compiler
	scope *scope
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

	// Push a new scope for this binding pass.
	b.PushScope()
	defer b.PopScope()

	// The binding logic is split into two-phases, due to the possibility of intra-stack references between elements.
	phase1 := &binderPhase1{b: b, doc: doc}
	phase2 := &binderPhase2{b: b, doc: doc}

	// Now walk the trees.  We use an InOrderVisitor to do this in the right order, handling determinism, etc. for us.
	v1 := core.NewInOrderVisitor(phase1, nil)
	v1.VisitStack(doc, stack)

	if b.Diag().Success() {
		v2 := core.NewInOrderVisitor(phase2, nil)
		v2.VisitStack(doc, stack)
	}
}

// LookupStack binds a name to a Stack type.
func (b *binder) LookupStack(nm ast.Name) (*Symbol, *ast.Stack) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupStack")
	return b.scope.LookupStack(nm)
}

// LookupSymbol binds a name to any kind of Symbol.
func (b *binder) LookupSymbol(nm ast.Name) *Symbol {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupSymbol")
	return b.scope.LookupSymbol(nm)
}

// RegisterSymbol registers a symbol with the given name; if it already exists, the function returns false.
func (b *binder) RegisterSymbol(sym *Symbol) bool {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during RegisterSymbol")
	return b.scope.RegisterSymbol(sym)
}

// PushScope creates a new scope with an empty symbol table parented to the existing one.
func (b *binder) PushScope() {
	b.scope = &scope{parent: b.scope, symtbl: make(map[ast.Name]*Symbol)}
}

// PopScope replaces the current scope with its parent.
func (b *binder) PopScope() {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during pop")
	b.scope = b.scope.parent
}

// scope enables lookups and symbols to obey traditional language scoping rules.
type scope struct {
	parent *scope
	symtbl map[ast.Name]*Symbol
}

// LookupStack binds a name to a Stack type.
func (s *scope) LookupStack(nm ast.Name) (*Symbol, *ast.Stack) {
	sym := s.LookupSymbol(nm)
	if sym != nil && sym.Kind == SymKindStack {
		return sym, sym.Real.(*ast.Stack)
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, nil
}

// LookupSymbol binds a name to any kind of Symbol.
func (s *scope) LookupSymbol(nm ast.Name) *Symbol {
	for s != nil {
		if sym, exists := s.symtbl[nm]; exists {
			return sym
		}
		s = s.parent
	}
	return nil
}

// RegisterSymbol registers a symbol with the given name; if it already exists, the function returns false.
func (s *scope) RegisterSymbol(sym *Symbol) bool {
	nm := sym.Name
	if _, exists := s.symtbl[nm]; exists {
		// TODO: this won't catch "shadowing" for parent scopes; do we care about this?
		return false
	}

	s.symtbl[nm] = sym
	return true
}

type binderPhase1 struct {
	b   *binder
	doc *diag.Document
}

var _ core.Visitor = &binderPhase1{} // compile-time assertion that the binder implements core.Visitor.

func (p *binderPhase1) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase1) VisitWorkspace(doc *diag.Document, workspace *ast.Workspace) {
}

func (p *binderPhase1) VisitCluster(doc *diag.Document, name string, cluster *ast.Cluster) {
}

func (p *binderPhase1) VisitDependency(doc *diag.Document, parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
	// Dependency versions must be valid semantic versions or ranges.  Ensure this parses.
	_, err := ref.Parse()
	if err != nil {
		p.Diag().Errorf(errors.ErrorMalformedStackReference.WithDocument(doc), ref, err)
	}
}

func (p *binderPhase1) VisitStack(doc *diag.Document, stack *ast.Stack) {
	// Stack names are required.
	if stack.Name == "" {
		p.Diag().Errorf(errors.ErrorMissingStackName.WithDocument(doc))
	}

	// Stack versions must be valid semantic versions (and specifically, not ranges).  In other words, we need
	// a concrete version number like "1.3.9-beta2" and *not* a range like ">1.3.9".
	// TODO: should we require a version number?
	if stack.Version != "" {
		if err := stack.Version.Check(); err != nil {
			p.Diag().Errorf(errors.ErrorIllegalStackVersion.WithDocument(doc), stack.Version, err)
		}
	}
}

func (p *binderPhase1) VisitProperty(doc *diag.Document, parent *ast.Stack, name string, param *ast.Property) {
}

func (p *binderPhase1) VisitBoundDependency(doc *diag.Document, parent *ast.Stack, dep *ast.BoundDependency) {
	// During the first phase of binding, we need to populate the symbol table with this Stack's dependencies, as well
	// as remember them on the AST for subsequent use (e.g., during code-generation).
	nm := dep.Ref.Name
	sym := NewStackSymbol(nm, dep.Stack)
	if !p.b.RegisterSymbol(sym) {
		p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.WithDocument(p.doc), nm)
	}

	// TODO: come up with some way of identifying unused dependencies and warning about them.
}

func (p *binderPhase1) VisitServices(doc *diag.Document, parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderPhase1) VisitService(doc *diag.Document, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
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
	simplify := false
	if svc.Type == "" {
		svc.Type = ast.Ref(svc.Name)
		simplify = true
	}

	// Now verify that the type is a valid stack reference.  If not, bail quickly.
	ty, err := svc.Type.Parse()
	if err != nil {
		p.Diag().Errorf(errors.ErrorMalformedStackReference.WithDocument(doc), svc.Type, err)
		return
	}

	// If we used the simple form, we must now propagate the friendly name over to the service's name.
	if simplify {
		svc.Name = ty.Name.Simple()
	}

	// Next, note that service definitions can "refer" to other service definitions within the same file.  Any
	// unqualified name is interpreted as such.  As a result, we must add this service to the symbol table even before
	// doing any subsequent binding of its type, etc.  This simplifies the 2nd phase of binding which can rely on this
	// fact, making its logic far simpler.
	sym := NewServiceSymbol(svc.Name, svc)
	if !p.b.RegisterSymbol(sym) {
		p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.WithDocument(p.doc), sym.Name)
	}
}

type binderPhase2 struct {
	b   *binder
	doc *diag.Document
}

var _ core.Visitor = &binderPhase2{} // compile-time assertion that the binder implements core.Visitor.

func (p *binderPhase2) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase2) VisitWorkspace(doc *diag.Document, workspace *ast.Workspace) {
}

func (p *binderPhase2) VisitCluster(doc *diag.Document, name string, cluster *ast.Cluster) {
}

func (p *binderPhase2) VisitDependency(doc *diag.Document, parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
}

func (p *binderPhase2) VisitStack(doc *diag.Document, stack *ast.Stack) {
	if stack.Base != "" {
		// Ensure the name of the base is in scope, and remember the binding information.
		if _, stack.BoundBase = p.b.LookupStack(stack.Base); stack.BoundBase == nil {
			p.Diag().Errorf(errors.ErrorTypeNotFound.WithDocument(doc), stack.Base)
		}
	}
	if !stack.Abstract && len(stack.Services.Public) == 0 && len(stack.Services.Private) == 0 {
		// Non-abstract Stacks must declare at least one Service.
		p.Diag().Errorf(errors.ErrorNonAbstractStacksMustDefineServices.WithDocument(doc))
	}
}

func (p *binderPhase2) VisitProperty(doc *diag.Document, parent *ast.Stack, name string, param *ast.Property) {
}

func (p *binderPhase2) VisitBoundDependency(doc *diag.Document, parent *ast.Stack, dep *ast.BoundDependency) {
}

func (p *binderPhase2) VisitServices(doc *diag.Document, parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderPhase2) VisitService(doc *diag.Document, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
	// The service's type has been prepared in phase 1, and must now be bound to a symbol.  All shorthand type
	// expressions, intra stack references, cycles, and so forth, will have been taken care of by this earlier phase.
	util.AssertMF(svc.Type != "",
		"Expected all Services to have types in binding phase2; %v is missing one", svc.Name)
	if ty, err := svc.Type.Parse(); err == nil {
		if _, svc.BoundType = p.b.LookupStack(ty.Name); svc.BoundType == nil {
			p.Diag().Errorf(errors.ErrorTypeNotFound.WithDocument(p.doc), svc.Type)
		}
	}
}
