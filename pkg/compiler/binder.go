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

	// PrepareStack prepares the AST for binding.  It returns a list of all unresolved dependency references.  These
	// must be bound and supplied to the BindStack function as the deps argument.
	PrepareStack(stack *ast.Stack) []ast.Ref
	// BindStack takes an AST, and its set of dependencies, and binds all names inside, mutating it in place.  It
	// returns a full list of all dependency Stacks that this Stack depends upon (which must then be bound).
	BindStack(stack *ast.Stack, depdocs ast.DependencyDocuments) []*ast.Stack
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

func (b *binder) PrepareStack(stack *ast.Stack) []ast.Ref {
	glog.Infof("Preparing Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer glog.V(2).Infof("Preparing Mu Stack %v completed w/ %v warnings and %v errors",
			stack.Name, b.Diag().Warnings(), b.Diag().Errors())
	}

	// Push a new scope for this binding pass.
	b.PushScope()

	// Now perform a phase1 walk of the tree, preparing it for subsequent binding.  This must be done as a
	// separate phase because we won't know what to stick into the symbol table until after this first walk.
	phase1 := newBinderPhase1(b, stack)
	v1 := core.NewInOrderVisitor(phase1, nil)
	v1.VisitStack(stack)

	// Return a set of dependency references that must be loaded before BindStack occurs.
	deprefs := make([]ast.Ref, 0, len(phase1.deps))
	for dep := range phase1.deps {
		deprefs = append(deprefs, dep)
	}
	return deprefs
}

func (b *binder) BindStack(stack *ast.Stack, depdocs ast.DependencyDocuments) []*ast.Stack {
	glog.Infof("Binding Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer glog.V(2).Infof("Binding Mu Stack %v completed w/ %v warnings and %v errors",
			stack.Name, b.Diag().Warnings(), b.Diag().Errors())
	}

	// Restore the original scope after this binding pass.
	defer b.PopScope()

	// Now perform a phase2 walk of the tree, completing the binding process.  The 1st walk will have given
	// us everything we need for a fully populated symbol table, so that type binding will resolve correctly.
	phase2 := newBinderPhase2(b, stack, depdocs)
	v2 := core.NewInOrderVisitor(phase2, nil)
	v2.VisitStack(stack)
	return phase2.deps
}

// LookupStack binds a name to a Stack type.
func (b *binder) LookupStack(nm ast.Name) (*ast.Stack, bool) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupStack")
	return b.scope.LookupStack(nm)
}

// LookupDocument binds a name to a Document type.
func (b *binder) LookupDocument(nm ast.Name) (*diag.Document, bool) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupDocument")
	return b.scope.LookupDocument(nm)
}

// LookupSymbol binds a name to any kind of Symbol.
func (b *binder) LookupSymbol(nm ast.Name) (*Symbol, bool) {
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
func (s *scope) LookupStack(nm ast.Name) (*ast.Stack, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == SymKindStack {
		return sym.Real.(*ast.Stack), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupDocument binds a name to a Document type.
func (s *scope) LookupDocument(nm ast.Name) (*diag.Document, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == SymKindDocument {
		return sym.Real.(*diag.Document), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupSymbol binds a name to any kind of Symbol.
func (s *scope) LookupSymbol(nm ast.Name) (*Symbol, bool) {
	for s != nil {
		if sym, exists := s.symtbl[nm]; exists {
			return sym, true
		}
		s = s.parent
	}
	return nil, false
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
	b    *binder
	top  *ast.Stack
	deps map[ast.Ref]bool
}

var _ core.Visitor = &binderPhase1{} // compile-time assertion that the binder implements core.Visitor.

func newBinderPhase1(b *binder, top *ast.Stack) *binderPhase1 {
	return &binderPhase1{b, top, make(map[ast.Ref]bool)}
}

func (p *binderPhase1) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase1) VisitWorkspace(workspace *ast.Workspace) {
}

func (p *binderPhase1) VisitCluster(name string, cluster *ast.Cluster) {
}

func (p *binderPhase1) VisitDependency(parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
	// Workspace dependencies must use legal version specs; validate that this parses now so that we can use it
	// later on without needing to worry about additional validation.
	_, err := ref.Parse()
	if err != nil {
		p.Diag().Errorf(errors.ErrorMalformedStackReference.At(parent), ref, err)
	}
}

// registerDependency adds a dependency that needs to be resolved/bound before phase 2 occurs.
func (p *binderPhase1) registerDependency(stack *ast.Stack, ref ast.Ref) (ast.RefParts, bool) {
	ty, err := ref.Parse()
	if err == nil {
		// First see if this resolves to a stack.  If it does, it's already in scope; nothing more to do.
		nm := ty.Name
		if _, exists := p.b.LookupStack(nm); !exists {
			// Otherwise, we need to track this as a dependency to resolve.  Make sure to canonicalize the key so that
			// we don't end up with duplicate semantically equivalent dependency references.
			key := ty.Defaults().Ref()
			p.deps[key] = true
		}

		return ty, true
	}

	p.Diag().Errorf(errors.ErrorMalformedStackReference.At(stack), ref, err)
	return ty, false
}

func (p *binderPhase1) VisitStack(stack *ast.Stack) {
	// If the stack has a base type, we must add it as a bound dependency.
	if stack.Base != "" {
		p.registerDependency(stack, stack.Base)
	}

	// Stack names are required.
	if stack.Name == "" {
		p.Diag().Errorf(errors.ErrorMissingStackName.At(stack))
	}

	// Stack versions must be valid semantic versions (and specifically, not ranges).  In other words, we need
	// a concrete version number like "1.3.9-beta2" and *not* a range like ">1.3.9".
	// TODO: should we require a version number?
	if stack.Version != "" {
		if err := stack.Version.Check(); err != nil {
			p.Diag().Errorf(errors.ErrorIllegalStackVersion.At(stack), stack.Version, err)
		}
	}
}

func (p *binderPhase1) VisitProperty(parent *ast.Stack, name string, param *ast.Property) {
}

func (p *binderPhase1) VisitServices(parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderPhase1) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name,
	public bool, svc *ast.Service) {
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

	// Remember this service's type as a stack that must be bound later on.
	ty, ok := p.registerDependency(pstack, svc.Type)
	if !ok {
		return
	}

	// If we used the simple form, we must now propagate the friendly name over to the service's name.
	if simplify {
		svc.Name = ty.Name.Simple()
	}

	// Add this service to the symbol table so that other service definitions can refer to it by name.
	sym := NewServiceSymbol(svc.Name, svc)
	if !p.b.RegisterSymbol(sym) {
		p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.At(pstack), sym.Name)
	}
}

type binderPhase2 struct {
	b    *binder
	top  *ast.Stack   // the top-most stack being bound.
	deps []*ast.Stack // a set of dependencies instantiated during this binding phase.
}

var _ core.Visitor = &binderPhase2{} // compile-time assertion that the binder implements core.Visitor.

func newBinderPhase2(b *binder, top *ast.Stack, depdocs ast.DependencyDocuments) *binderPhase2 {
	p := &binderPhase2{b: b, top: top}

	// Populate the symbol table with this Stack's bound dependencies so that any type lookups are found.
	for _, ref := range ast.StableDependencyDocuments(depdocs) {
		doc := depdocs[ref]
		util.Assert(doc != nil)

		nm := refToName(ref)
		sym := NewDocumentSymbol(nm, doc)
		if !p.b.RegisterSymbol(sym) {
			p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.At(doc), nm)
		}
	}

	return p
}

func (p *binderPhase2) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPhase2) VisitWorkspace(workspace *ast.Workspace) {
}

func (p *binderPhase2) VisitCluster(name string, cluster *ast.Cluster) {
}

func (p *binderPhase2) VisitDependency(parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
}

func (p *binderPhase2) VisitStack(stack *ast.Stack) {
	// Ensure the name of the base is in scope, and remember the binding information.
	if stack.Base != "" {
		// TODO[marapongo/mu#7]: we need to plumb construction properties for this stack.
		stack.BoundBase = p.ensureStackType(stack.Base, nil)
	}

	// Non-abstract Stacks must declare at least one Service.
	if !stack.Predef && !stack.Abstract && len(stack.Services.Public) == 0 && len(stack.Services.Private) == 0 {
		p.Diag().Errorf(errors.ErrorNonAbstractStacksMustDefineServices.At(stack))
	}
}

func (p *binderPhase2) VisitProperty(parent *ast.Stack, name string, param *ast.Property) {
}

func (p *binderPhase2) VisitServices(parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderPhase2) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
	// The service's type has been prepared in phase 1, and must now be bound to a symbol.  All shorthand type
	// expressions, intra stack references, cycles, and so forth, will have been taken care of by this earlier phase.
	util.AssertMF(svc.Type != "",
		"Expected all Services to have types in binding phase2; %v is missing one", svc.Name)
	svc.BoundType = p.ensureStackType(svc.Type, svc.Props)
}

// refToName converts a reference to its simple symbolic name.
func refToName(ref ast.Ref) ast.Name {
	return ref.MustParse().Name
}

func (p *binderPhase2) ensureStackType(ref ast.Ref, props ast.PropertyBag) *ast.Stack {
	// There are two possibilities.  The first is that a type resolves to an *ast.Stack.  That's simple, we just fetch
	// and return it.  The second is that a type resolves to a *diag.Document.  That's more complex, as we need to
	// actually parse the stack from a document, supplying properties, etc., for template expansion.
	nm := refToName(ref)
	stack, exists := p.b.LookupStack(nm)
	if exists {
		return stack
	}

	doc, exists := p.b.LookupDocument(nm)
	if exists {
		// If we got this far, we've loaded up the dependency's Mufile; parse it and return the result.  Note that
		// this will be processed later on in semantic analysis, to ensure semantic problems are caught.
		p := NewParser(p.b.c)
		stack := p.ParseStack(doc, props)
		if p.Diag().Success() {
			return stack
		} else {
			return nil
		}
	}

	util.FailMF("Expected 1st pass of binding to guarantee type %v exists (%v)", ref, nm)
	return nil
}
