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

	// PrepareStack prepares the AST for binding.
	PrepareStack(stack *ast.Stack)
	// BindStack takes an AST and binds all names inside of it, mutating it in place.
	BindStack(stack *ast.Stack)
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

func (b *binder) PrepareStack(stack *ast.Stack) {
	glog.Infof("Preparing Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Preparing Mu Stack %v completed w/ %v warnings and %v errors",
				stack.Name, b.Diag().Warnings(), b.Diag().Errors())
		}()
	}

	// Push a new scope for this binding pass.
	b.PushScope()

	// Now perform a phase1 walk of the tree, preparing it for subsequent binding.  This must be done as a
	// separate phase because we won't know what to stick into the symbol table until after this first walk.
	phase1 := &binderPhase1{b: b, top: stack}
	v1 := core.NewInOrderVisitor(phase1, nil)
	v1.VisitStack(stack)
}

func (b *binder) BindStack(stack *ast.Stack) {
	glog.Infof("Binding Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Binding Mu Stack %v completed w/ %v warnings and %v errors",
				stack.Name, b.Diag().Warnings(), b.Diag().Errors())
		}()
	}

	// Restore the original scope after this binding pass.
	defer b.PopScope()

	// Now perform a phase2 walk of the tree, completing the binding process.  The 1st walk will have given
	// us everything we need for a fully populated symbol table, so that type binding will resolve correctly.
	phase2 := &binderPhase2{b: b, top: stack}
	v2 := core.NewInOrderVisitor(phase2, nil)
	v2.VisitStack(stack)
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
	top *ast.Stack
}

var _ core.Visitor = &binderPhase1{} // compile-time assertion that the binder implements core.Visitor.

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
		p.Diag().Errorf(errors.ErrorMalformedStackReference.WithDocument(parent.Doc), ref, err)
	}
}

// registerDependencyToBind adds a dependency that needs to be resolved/bound before phase 2 occurs.
func (p *binderPhase1) registerDependencyToBind(stack *ast.Stack, ref ast.Ref) (ast.RefParts, bool) {
	util.Assert(stack.BoundDependencies != nil)

	ty, err := ref.Parse()
	if err == nil {
		// Ensure that empty RefParts are canonicalized in the key we use so we don't add duplicates.
		ref = ty.Defaults().Ref()

		if _, exist := stack.BoundDependencies[ref]; !exist {
			// If a stack symbol exists for this name, use it so that the compiler needn't resolve it.  Else, nil.
			nm := ty.Name
			_, st := p.b.LookupStack(nm)

			stack.BoundDependencies[ref] = ast.BoundDependency{
				Ref:   ty,
				Stack: st,
			}
		}

		return ty, true
	}

	p.Diag().Errorf(errors.ErrorMalformedStackReference.WithDocument(stack.Doc), ref, err)
	return ty, false
}

func (p *binderPhase1) VisitStack(stack *ast.Stack) {
	// Make room for bound dependencies.
	util.Assert(stack.BoundDependencies == nil)
	stack.BoundDependencies = make(ast.BoundDependencies)

	// If the stack has a base type, we must add it as a bound dependency.
	if stack.Base != "" {
		p.registerDependencyToBind(stack, stack.Base)
	}

	// Stack names are required.
	if stack.Name == "" {
		p.Diag().Errorf(errors.ErrorMissingStackName.WithDocument(stack.Doc))
	}

	// Stack versions must be valid semantic versions (and specifically, not ranges).  In other words, we need
	// a concrete version number like "1.3.9-beta2" and *not* a range like ">1.3.9".
	// TODO: should we require a version number?
	if stack.Version != "" {
		if err := stack.Version.Check(); err != nil {
			p.Diag().Errorf(errors.ErrorIllegalStackVersion.WithDocument(stack.Doc), stack.Version, err)
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
	ty, ok := p.registerDependencyToBind(pstack, svc.Type)
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
		p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.WithDocument(pstack.Doc), sym.Name)
	}
}

type binderPhase2 struct {
	b   *binder
	top *ast.Stack
}

var _ core.Visitor = &binderPhase2{} // compile-time assertion that the binder implements core.Visitor.

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
	// Populate the symbol table with this Stack's bound dependencies so that any type lookups are found.
	util.Assert(stack.BoundDependencies != nil)
	for _, nm := range ast.StableBoundDependencies(stack.BoundDependencies) {
		dep := stack.BoundDependencies[nm]
		util.Assert(dep.Stack != nil)
		sym := NewStackSymbol(dep.Ref.Name, dep.Stack)
		if !p.b.RegisterSymbol(sym) {
			p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.WithDocument(stack.Doc), dep.Ref.Name)
		}
	}

	// Ensure the name of the base is in scope, and remember the binding information.
	if stack.Base != "" {
		nm := stack.Base.MustParse().Name
		_, stack.BoundBase = p.b.LookupStack(nm)
		util.AssertMF(stack.BoundBase != nil, "Expected 1st pass of binding to guarantee %v exists", nm)
	}

	// Non-abstract Stacks must declare at least one Service.
	if !stack.Predef && !stack.Abstract && len(stack.Services.Public) == 0 && len(stack.Services.Private) == 0 {
		p.Diag().Errorf(errors.ErrorNonAbstractStacksMustDefineServices.WithDocument(stack.Doc))
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
	nm := svc.Type.MustParse().Name
	_, svc.BoundType = p.b.LookupStack(nm)
	util.AssertMF(svc.BoundType != nil, "Expected 1st pass of binding to guarantee %v exists", nm)
}
