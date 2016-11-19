// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/predef"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
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
	for nm, stack := range predef.StackTypes {
		sym := NewStackSymbol(nm, stack)
		if ok := b.RegisterSymbol(sym); !ok {
			glog.Fatalf("Unexpected Symbol collision when registering predef Stack type %v\n", nm)
		}
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

	if b.Diag().Errors() == 0 {
		v2 := core.NewInOrderVisitor(phase2, nil)
		v2.VisitStack(doc, stack)
	}
}

// LookupStack binds a name to a Stack type.
func (b *binder) LookupStack(nm ast.Name) (*Symbol, *ast.Stack) {
	if b.scope == nil {
		glog.Fatalf("Unexpected empty binding scope during LookupStack")
	}
	return b.scope.LookupStack(nm)
}

// LookupSymbol binds a name to any kind of Symbol.
func (b *binder) LookupSymbol(nm ast.Name) *Symbol {
	if b.scope == nil {
		glog.Fatalf("Unexpected empty binding scope during LookupSymbol")
	}
	return b.scope.LookupSymbol(nm)
}

// RegisterSymbol registers a symbol with the given name; if it already exists, the function returns false.
func (b *binder) RegisterSymbol(sym *Symbol) bool {
	if b.scope == nil {
		glog.Fatalf("Unexpected empty binding scope during RegisterSymbol")
	}
	return b.scope.RegisterSymbol(sym)
}

// PushScope creates a new scope with an empty symbol table parented to the existing one.
func (b *binder) PushScope() {
	b.scope = &scope{parent: b.scope, symtbl: make(map[ast.Name]*Symbol)}
}

// PopScope replaces the current scope with its parent.
func (b *binder) PopScope() {
	if b.scope == nil {
		glog.Fatalf("Unexpected empty binding scope during pop")
	}
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
	core.Visitor
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
	// TODO: populate the symbol table with each dependency's stack object.
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

func (p *binderPhase1) VisitTarget(doc *diag.Document, name string, target *ast.Target) {
}

type binderPhase2 struct {
	core.Visitor
	b   *binder
	doc *diag.Document
}

func (p *binderPhase2) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase2) VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata) {
}

func (p *binderPhase2) VisitStack(doc *diag.Document, stack *ast.Stack) {
	if stack.Base != "" {
		// Ensure the name of the base is in scope, and remember the binding information.
		_, stack.BoundBase = p.b.LookupStack(stack.Base)
		if stack.BoundBase == nil {
			p.Diag().Errorf(errors.TypeNotFound.WithDocument(doc), stack.Base)
		}
	}
	if !stack.Abstract && len(stack.Services.Public) == 0 && len(stack.Services.Private) == 0 {
		// Non-abstract Stacks must declare at least one Service.
		p.Diag().Errorf(errors.NonAbstractStacksMustDefineServices.WithDocument(doc))
	}
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
	if svc.Type == "" {
		glog.Fatalf("Expected all Services to have types in binding phase2; %v is missing one", svc.Name)
	}
	ty, _ := p.b.LookupStack(svc.Type)
	if ty == nil {
		p.Diag().Errorf(errors.TypeNotFound.WithDocument(p.doc), svc.Type)
	}
}

func (p *binderPhase2) VisitTarget(doc *diag.Document, name string, target *ast.Target) {
}
