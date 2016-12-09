// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/ast/conv"
	"github.com/marapongo/mu/pkg/compiler/core"
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
	BindStack(stack *ast.Stack, deprefs ast.DependencyRefs) []*ast.Stack
	// ValidateStack runs last, after all transitive dependencies have been bound, to perform last minute validation.
	ValidateStack(stack *ast.Stack)
}

func NewBinder(c Compiler) Binder {
	// Create a new binder and a new scope with an empty symbol table.
	b := &binder{c: c}
	b.PushScope()
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
	phase := newBinderPreparePhase(b, stack)
	v := core.NewInOrderVisitor(phase, nil)
	v.VisitStack(stack)

	// Return a set of dependency references that must be loaded before BindStack occurs.
	return phase.deps
}

func (b *binder) BindStack(stack *ast.Stack, deprefs ast.DependencyRefs) []*ast.Stack {
	glog.Infof("Binding Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer glog.V(2).Infof("Binding Mu Stack %v completed w/ %v warnings and %v errors",
			stack.Name, b.Diag().Warnings(), b.Diag().Errors())
	}

	// Now perform a phase2 walk of the tree, completing the binding process.  The 1st walk will have given
	// us everything we need for a fully populated symbol table, so that type binding will resolve correctly.
	phase := newBinderBindPhase(b, stack, deprefs)
	v := core.NewInOrderVisitor(phase, nil)
	v.VisitStack(stack)
	return phase.deps
}

func (b *binder) ValidateStack(stack *ast.Stack) {
	glog.Infof("Validating Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer glog.V(2).Infof("Validating Mu Stack %v completed w/ %v warnings and %v errors",
			stack.Name, b.Diag().Warnings(), b.Diag().Errors())
	}

	// Restore the original scope after this binding pass.
	defer b.PopScope()

	// Now perform the final validation of the AST.  There's nothing to return, it just may issue errors.
	phase := newBinderValidatePhase(b)
	v := core.NewInOrderVisitor(phase, nil)
	v.VisitStack(stack)
}

// LookupService binds a name to a Service type.
func (b *binder) LookupService(nm ast.Name) (*ast.Service, bool) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupService")
	return b.scope.LookupService(nm)
}

// LookupStack binds a name to a Stack type.
func (b *binder) LookupStack(nm ast.Name) (*ast.Stack, bool) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupStack")
	return b.scope.LookupStack(nm)
}

// LookupUninstStack binds a name to a UninstStack type.
func (b *binder) LookupUninstStack(nm ast.Name) (*ast.UninstStack, bool) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupUninstStack")
	return b.scope.LookupUninstStack(nm)
}

// LookupSchema binds a name to a Schema type.
func (b *binder) LookupSchema(nm ast.Name) (*ast.Schema, bool) {
	util.AssertM(b.scope != nil, "Unexpected empty binding scope during LookupSchema")
	return b.scope.LookupSchema(nm)
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

// LookupService binds a name to a Service type.
func (s *scope) LookupService(nm ast.Name) (*ast.Service, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == SymKindService {
		return sym.Real.(*ast.Service), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
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

// LookupUninstStack binds a name to a UninstStack type.
func (s *scope) LookupUninstStack(nm ast.Name) (*ast.UninstStack, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == SymKindUninstStack {
		return sym.Real.(*ast.UninstStack), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupSchema binds a name to a Schema type.
func (s *scope) LookupSchema(nm ast.Name) (*ast.Schema, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == SymKindSchema {
		return sym.Real.(*ast.Schema), true
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

type binderPreparePhase struct {
	b     *binder
	top   *ast.Stack
	deps  []ast.Ref
	depsm map[ast.Ref]bool
}

var _ core.Visitor = (*binderPreparePhase)(nil) // compile-time assertion that the binder implements core.Visitor.

func newBinderPreparePhase(b *binder, top *ast.Stack) *binderPreparePhase {
	return &binderPreparePhase{
		b:     b,
		top:   top,
		deps:  make([]ast.Ref, 0),
		depsm: make(map[ast.Ref]bool),
	}
}

func (p *binderPreparePhase) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderPreparePhase) VisitWorkspace(workspace *ast.Workspace) {
}

func (p *binderPreparePhase) VisitCluster(name string, cluster *ast.Cluster) {
}

func (p *binderPreparePhase) VisitDependency(parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
	// Workspace dependencies must use legal version specs; validate that this parses now so that we can use it
	// later on without needing to worry about additional validation.
	_, err := ref.Parse()
	if err != nil {
		p.Diag().Errorf(errors.ErrorIllegalNameLikeSyntax.At(parent), ref, err)
	}
}

func (p *binderPreparePhase) VisitStack(stack *ast.Stack) {
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

func (p *binderPreparePhase) VisitSchemas(parent *ast.Stack, schemas *ast.Schemas) {
}

func (p *binderPreparePhase) VisitSchema(pstack *ast.Stack, parent *ast.Schemas, name ast.Name,
	public bool, schema *ast.Schema) {

	// If the schema has an unresolved base type, add it as a bound dependency.
	if schema.BoundBase != nil && schema.BoundBase.IsUnresolvedRef() {
		p.registerDependency(pstack, *schema.BoundBase.Unref)
	}

	// Add this schema to the symbol table so that this stack can reference it.
	sym := NewSchemaSymbol(schema.Name, schema)
	if !p.b.RegisterSymbol(sym) {
		p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.At(pstack), sym.Name)
	}
}

func (p *binderPreparePhase) VisitProperty(parent *ast.Stack, schema *ast.Schema, name string, prop *ast.Property) {
	// For properties whose types represent stack types, register them as a dependency.
	if prop.BoundType.IsUnresolvedRef() {
		p.registerDependency(parent, *prop.BoundType.Unref)
	}
}

func (p *binderPreparePhase) VisitServices(parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderPreparePhase) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name,
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

// registerDependency adds a dependency that needs to be resolved/bound before phase 2 occurs.
func (p *binderPreparePhase) registerDependency(stack *ast.Stack, ref ast.Ref) (ast.RefParts, bool) {
	ty, err := ref.Parse()
	if err == nil {
		// First see if this resolves to a stack.  If it does, it's already in scope; nothing more to do.
		nm := ty.Name
		if _, exists := p.b.LookupStack(nm); !exists {
			// Otherwise, we need to track this as a dependency to resolve.  Make sure to canonicalize the key so that
			// we don't end up with duplicate semantically equivalent dependency references.
			key := ty.Defaults().Ref()
			if _, exist := p.depsm[key]; !exist {
				// Store these in an array so that the order is deterministic.  But use a map to avoid duplicates.
				p.deps = append(p.deps, key)
				p.depsm[key] = true
			}
		}

		return ty, true
	}

	p.Diag().Errorf(errors.ErrorIllegalNameLikeSyntax.At(stack), ref, err)
	return ty, false
}

type binderBindPhase struct {
	b    *binder
	top  *ast.Stack   // the top-most stack being bound.
	deps []*ast.Stack // a set of dependencies instantiated during this binding phase.
}

var _ core.Visitor = (*binderBindPhase)(nil) // compile-time assertion that the binder implements core.Visitor.

func newBinderBindPhase(b *binder, top *ast.Stack, deprefs ast.DependencyRefs) *binderBindPhase {
	p := &binderBindPhase{b: b, top: top}

	// Populate the symbol table with this Stack's bound dependencies so that any type lookups are found.
	for _, ref := range ast.StableDependencyRefs(deprefs) {
		dep := deprefs[ref]
		util.Assert(dep.Doc != nil)

		nm := refToName(ref)
		sym := NewUninstStackSymbol(nm, dep)
		if !p.b.RegisterSymbol(sym) {
			p.Diag().Errorf(errors.ErrorSymbolAlreadyExists.At(dep.Doc), nm)
		}
	}

	return p
}

func (p *binderBindPhase) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderBindPhase) VisitWorkspace(workspace *ast.Workspace) {
}

func (p *binderBindPhase) VisitCluster(name string, cluster *ast.Cluster) {
}

func (p *binderBindPhase) VisitDependency(parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
}

func (p *binderBindPhase) VisitStack(stack *ast.Stack) {
	// Ensure the name of the base is in scope, and remember the binding information.
	if stack.Base != "" {
		// TODO[marapongo/mu#7]: we need to plumb construction properties for this stack.
		stack.BoundBase = p.ensureStack(stack.Base, nil)
	}

	// Non-abstract Stacks must declare at least one Service.
	if !stack.Intrinsic && !stack.Abstract && len(stack.Services.Public) == 0 && len(stack.Services.Private) == 0 {
		p.Diag().Errorf(errors.ErrorNonAbstractStacksMustDefineServices.At(stack))
	}
}

func (p *binderBindPhase) VisitSchemas(parent *ast.Stack, schemas *ast.Schemas) {
}

func (p *binderBindPhase) VisitSchema(pstack *ast.Stack, parent *ast.Schemas, name ast.Name,
	public bool, schema *ast.Schema) {

	// Ensure the base schema is available to us.
	if schema.BoundBase != nil && schema.BoundBase.IsUnresolvedRef() {
		ref := *schema.BoundBase.Unref
		base := p.ensureType(ref)
		// Check to ensure that the base is of one of the legal kinds.
		if !base.IsPrimitive() && !base.IsSchema() {
			p.Diag().Errorf(errors.ErrorSchemaTypeExpected, ref, base)
		}
	}

	// TODO: ensure that schemas with constraints don't have illegal constraints (wrong type; regex won't parse; etc).
}

func (p *binderBindPhase) VisitProperty(parent *ast.Stack, schema *ast.Schema, name string, prop *ast.Property) {
	// For properties whose types represent unresolved names, we must bind them to a name now.
	if prop.BoundType.IsUnresolvedRef() {
		prop.BoundType = p.ensureType(*prop.BoundType.Unref)
		util.Assert(prop.BoundType != nil)
	}
}

func (p *binderBindPhase) VisitServices(parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderBindPhase) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
	// The service's type has been prepared in phase 1, and must now be bound to a symbol.  All shorthand type
	// expressions, intra stack references, cycles, and so forth, will have been taken care of by this earlier phase.
	util.AssertMF(svc.Type != "",
		"Expected all Services to have types in binding phase2; %v is missing one", svc.Name)
	svc.BoundType = p.ensureStack(svc.Type, svc.Properties)

	// A service cannot instantiate an abstract stack.
	if svc.BoundType != nil && svc.BoundType.Abstract {
		p.Diag().Errorf(errors.ErrorCannotCreateAbstractStack.At(pstack), svc.Name, svc.BoundType.Name)
	}
}

// ensureStack binds a ref to a stack symbol, possibly instantiating it if needed.
func (p *binderBindPhase) ensureStack(ref ast.Ref, props ast.PropertyBag) *ast.Stack {
	ty := p.ensureType(ref)

	// There are two possibilities.  The first is that a type resolves to an *ast.Stack.  That's simple, we just fetch
	// and return it.  The second is that a type resolves to a *diag.Document.  That's more complex, as we need to
	// actually parse the stack from a document, supplying properties, etc., for template expansion.
	if ty.IsStack() {
		return ty.Stack
	} else if ty.IsUninstStack() {
		// We have the dependency's Mufile; now we must "instantiate it", by parsing it and returning the result.  Note
		// that this will be processed later on in semantic analysis, to ensure semantic problems are caught.
		pa := NewParser(p.b.c)
		stack := pa.ParseStack(ty.UninstStack.Doc, props)
		if !pa.Diag().Success() {
			// If we failed to parse the stack, there was something wrong with our dependency information.  Bail out.
			return nil
		}
		p.deps = append(p.deps, stack)
		return stack
	} else {
		p.Diag().Errorf(errors.ErrorStackTypeExpected, ref, ty)
		return nil
	}
}

// ensureStackType looks up a ref, either as a stack, document, or schema symbol, and returns it as-is.
func (p *binderBindPhase) ensureType(ref ast.Ref) *ast.Type {
	nm := refToName(ref)
	stack, exists := p.b.LookupStack(nm)
	if exists {
		return ast.NewStackType(stack)
	}
	stref, exists := p.b.LookupUninstStack(nm)
	if exists {
		return ast.NewUninstStackType(stref)
	}
	schema, exists := p.b.LookupSchema(nm)
	if exists {
		return ast.NewSchemaType(schema)
	}
	util.FailMF("Expected 1st pass of binding to guarantee type %v exists (%v)", ref, nm)
	return nil
}

type binderValidatePhase struct {
	b *binder
}

var _ core.Visitor = (*binderValidatePhase)(nil) // compile-time assertion that the binder implements core.Visitor.

func newBinderValidatePhase(b *binder) *binderValidatePhase {
	return &binderValidatePhase{b: b}
}

func (p *binderValidatePhase) Diag() diag.Sink {
	return p.b.Diag()
}

func (p *binderValidatePhase) VisitWorkspace(workspace *ast.Workspace) {
}

func (p *binderValidatePhase) VisitCluster(name string, cluster *ast.Cluster) {
}

func (p *binderValidatePhase) VisitDependency(parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
}

func (p *binderValidatePhase) VisitStack(stack *ast.Stack) {
	if stack.PropertyValues != nil {
		// Bind property values.
		stack.BoundPropertyValues = p.bindProperties(&stack.Node, stack.Properties, stack.PropertyValues)
	}
	if stack.Base != "" {
		util.Assert(stack.BoundBase != nil)
		// TODO[marapongo/mu#7]: validate the properties from this stack on the base.
	}
}

func (p *binderValidatePhase) VisitSchemas(parent *ast.Stack, schemas *ast.Schemas) {
}

func (p *binderValidatePhase) VisitSchema(pstack *ast.Stack, parent *ast.Schemas, name ast.Name,
	public bool, schema *ast.Schema) {
}

func (p *binderValidatePhase) VisitProperty(parent *ast.Stack, schema *ast.Schema, name string, prop *ast.Property) {
}

func (p *binderValidatePhase) VisitServices(parent *ast.Stack, svcs *ast.Services) {
}

func (p *binderValidatePhase) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
	util.Assert(svc.BoundType != nil)
	if svc.BoundType.PropertyValues == nil {
		// For some types, there aren't any property values (e.g., built-in types).  For those, bind now.
		svc.BoundProperties = p.bindProperties(&pstack.Node, svc.BoundType.Properties, svc.Properties)
	} else {
		// For imported types, we should have property values, which already got bound in an earlier phase.
		util.Assert(svc.BoundType.BoundPropertyValues != nil)
		util.Assert(len(svc.BoundType.PropertyValues) == len(svc.Properties))
		svc.BoundProperties = svc.BoundType.BoundPropertyValues
	}
}

// bindProperties typechecks a set of unbounded properties against the target stack, and expands them into a bag
// of bound properties (with AST nodes rather than the naked parsed types).
func (p *binderValidatePhase) bindProperties(node *ast.Node, props ast.Properties,
	vals ast.PropertyBag) ast.LiteralPropertyBag {
	bound := make(ast.LiteralPropertyBag)

	// First, enumerate all known properties on the stack.  Ensure all required properties are present, expand default
	// values for missing ones where applicable, and check that types are correct, converting them as appropriate.
	for _, pname := range ast.StableProperties(props) {
		prop := props[pname]

		// First see if a value has been supplied by the caller.
		val, has := vals[pname]
		if !has || val == nil {
			if prop.Default != nil {
				// If the property has a default value, stick it in and process it normally.
				val = prop.Default
			} else if prop.Optional {
				// If this is an optional property, ok, just skip the remainder of processing.
				continue
			} else {
				// If there's no value, no default, and it isn't optional, issue an error and move on.
				p.Diag().Errorf(errors.ErrorMissingRequiredProperty.At(node), pname)
				continue
			}
		}
		util.Assert(val != nil)

		// Now, value in hand, let's make sure it's the right type.
		if lit := p.bindValue(&prop.Node, val, prop.BoundType); lit != nil {
			bound[pname] = lit
		}
	}

	for _, pname := range ast.StablePropertyBag(vals) {
		if _, ok := props[pname]; !ok {
			// TODO: edit distance checking to help with suggesting a fix.
			p.Diag().Errorf(errors.ErrorUnrecognizedProperty.At(node), pname)
		}
	}

	return bound
}

// bindValue takes a value and binds it to a type and literal AST node, returning nils if the conversions fails.
func (p *binderValidatePhase) bindValue(node *ast.Node, val interface{}, ty *ast.Type) ast.Literal {
	util.Assert(ty != nil)
	var lit ast.Literal
	if ty.IsDecors() {
		lit = p.bindDecorsValue(node, val, ty.Decors)
	} else if ty.IsPrimitive() {
		lit = p.bindPrimitiveValue(node, val, *ty.Primitive)
	} else if ty.IsStack() {
		lit = p.bindServiceValue(node, val, ty)
	} else if ty.IsSchema() {
		lit = p.bindSchemaValue(node, val, ty.Schema)
	} else if ty.IsUnresolvedRef() {
		util.FailM("Expected all unresolved refs to be gone by this phase in binding")
	}

	if lit == nil {
		// If no successful type binding happened, issue an error.
		p.Diag().Errorf(errors.ErrorIncorrectType.At(node), ty, reflect.TypeOf(val))
	}
	return lit
}

func (p *binderValidatePhase) bindDecorsValue(node *ast.Node, val interface{}, decors *ast.TypeDecors) ast.Literal {
	// For decorated types, we need to recurse.
	if decors.ElemType != nil {
		arr := reflect.ValueOf(val)
		if arr.Kind() == reflect.Slice {
			len := arr.Len()
			lits := make([]ast.Literal, len)
			err := false
			for i := 0; i < len; i++ {
				v := arr.Index(i).Interface()
				if lits[i] = p.bindValue(node, v, decors.ElemType); lits[i] == nil {
					err = true
				}
			}
			if !err {
				return ast.NewArrayLiteral(node, decors.ElemType, lits)
			}
		} else {
			glog.V(7).Infof("Expected array for value %v, got %v", val, arr.Kind())
		}
	} else {
		util.Assert(decors.KeyType != nil)
		util.Assert(decors.ValueType != nil)

		// TODO: ensure that keytype is something we can actually use as a key (primitive).

		m := reflect.ValueOf(val)
		if m.Kind() == reflect.Map {
			mk := m.MapKeys()
			keys := make([]ast.Literal, len(mk))
			err := false
			for i := 0; i < len(mk); i++ {
				k := mk[i].Interface()
				if keys[i] = p.bindValue(node, k, decors.KeyType); keys[i] == nil {
					glog.V(7).Infof("Error binding map key #%v (%v); expected %v",
						i, k, decors.KeyType)
					err = true
				}
			}
			vals := make([]ast.Literal, len(mk))
			for i := 0; i < len(mk); i++ {
				v := m.MapIndex(mk[i])
				if vals[i] = p.bindValue(node, v, decors.ValueType); vals[i] == nil {
					glog.V(7).Infof("Error binding map value #%v (k=%v v=%v); expected %v",
						i, mk[i].Interface(), v, decors.ValueType)
					err = true
				}
			}
			if !err {
				return ast.NewMapLiteral(node, decors.KeyType, decors.ValueType, keys, vals)
			}
		} else {
			glog.V(7).Infof("Expected map for value %v, got %v", val, m.Kind())
		}
	}

	return nil
}

func (p *binderValidatePhase) bindPrimitiveValue(node *ast.Node, val interface{}, prim ast.PrimitiveType) ast.Literal {
	// For primitive types, simply cast the target to the expected type.
	switch prim {
	case ast.PrimitiveTypeAny:
		// Any is easy: just store it as-is.
		return ast.NewAnyLiteral(node, val)
	case ast.PrimitiveTypeString:
		if s, ok := val.(string); ok {
			return ast.NewStringLiteral(node, s)
		}
		return nil
	case ast.PrimitiveTypeNumber:
		if n, ok := val.(float64); ok {
			return ast.NewNumberLiteral(node, n)
		}
		return nil
	case ast.PrimitiveTypeBool:
		if b, ok := val.(bool); ok {
			return ast.NewBoolLiteral(node, b)
		}
		return nil
	case ast.PrimitiveTypeService:
		// Extract the name of the service reference as a string.  Then bind it to an actual service in our symbol
		// table, and store a strong reference to the result.  This lets the backend connect the dots.
		return p.bindServiceValue(node, val, nil)
	default:
		util.FailMF("Unrecognized primitive type: %v", prim)
		return nil
	}
}

func (p *binderValidatePhase) bindServiceValue(node *ast.Node, val interface{}, expect *ast.Type) ast.Literal {
	// Bind the capability ref for this stack type.
	if s, ok := val.(string); ok {
		if ref := p.bindServiceRef(node, s, expect); ref != nil {
			return ast.NewServiceLiteral(node, ref)
		}
	}
	return nil
}

func (p *binderValidatePhase) bindSchemaValue(node *ast.Node, val interface{}, schema *ast.Schema) ast.Literal {
	// Bind the custom schema type.  This is rather involved, but there are two primary cases:
	//     1) A base type exists, plus an optional set of constraints on that base type (if it's a primitive).
	//     2) A set of properties exist, meaning an entirely custom object.  We must go recursive.
	// TODO[marapongo/mu#9]: we may want to support mixing these (e.g., additive properties); for now, we won't.
	if schema.BoundBase != nil {
		// There is a base type.  Bind it as-is, and then apply any additional constraints we have added.
		util.Assert(schema.Properties == nil)
		lit := p.bindValue(node, val, schema.BoundBase)
		if lit != nil {
			// The following constraints are valid only on strings:
			if schema.Pattern != nil {
				if s, ok := conv.ToString(lit); ok {
					rex := regexp.MustCompile(*schema.Pattern)
					if rex.FindString(s) != s {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet, "pattern", schema.Pattern, s)
					}
				} else {
					p.Diag().Errorf(errors.ErrorSchemaConstraintType, "maxLength", ast.PrimitiveTypeString, lit.Type())
				}
			}
			if schema.MaxLength != nil {
				if s, ok := conv.ToString(lit); ok {
					c := utf8.RuneCountInString(s)
					if float64(c) > *schema.MaxLength {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet,
							"maxLength", fmt.Sprintf("max %v", schema.MaxLength), c)
					}
				} else {
					p.Diag().Errorf(errors.ErrorSchemaConstraintType, "maxLength", ast.PrimitiveTypeString, lit.Type())
				}
			}
			if schema.MinLength != nil {
				if s, ok := conv.ToString(lit); ok {
					c := utf8.RuneCountInString(s)
					if float64(c) < *schema.MinLength {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet,
							"minLength", fmt.Sprintf("min %v", schema.MinLength), c)
					}
				} else {
					p.Diag().Errorf(errors.ErrorSchemaConstraintType, "minLength", ast.PrimitiveTypeString, lit.Type())
				}
			}

			// The following constraints are valid only on numeric ypes:
			if schema.Maximum != nil {
				if n, ok := conv.ToNumber(lit); ok {
					if n > *schema.Maximum {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet,
							"maximum", fmt.Sprintf("max %v", schema.Maximum), n)
					}
				} else {
					p.Diag().Errorf(errors.ErrorSchemaConstraintType, "maximum", ast.PrimitiveTypeNumber, lit.Type())
				}
			}
			if schema.Minimum != nil {
				if n, ok := conv.ToNumber(lit); ok {
					if n < *schema.Minimum {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet,
							"minimum", fmt.Sprintf("min %v", schema.Minimum), n)
					}
				} else {
					p.Diag().Errorf(errors.ErrorSchemaConstraintType, "minimum", ast.PrimitiveTypeNumber, lit.Type())
				}
			}

			// The following constraints are valid on strings *and* number types.
			if len(schema.Enum) > 0 {
				if s, ok := conv.ToString(lit); ok {
					ok := false
					for _, e := range schema.Enum {
						if s == e.(string) {
							ok = true
							break
						}
					}
					if !ok {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet,
							"enum", fmt.Sprintf("enum %v", schema.Enum), s)
					}
				} else if n, ok := conv.ToNumber(lit); ok {
					ok := false
					for _, e := range schema.Enum {
						if n == e.(float64) {
							ok = true
							break
						}
					}
					if !ok {
						p.Diag().Errorf(errors.ErrorSchemaConstraintUnmet,
							"enum", fmt.Sprintf("enum %v", schema.Enum), n)
					}
				} else {
					p.Diag().Errorf(errors.ErrorSchemaConstraintType,
						"enum", ast.PrimitiveTypeString+" or "+ast.PrimitiveTypeNumber, lit.Type())
				}
			}
		}
	} else if schema.Properties != nil {
		// There are some properties.  This is a custom type.  Bind the properties as usual.
		if props, ok := val.(ast.PropertyBag); ok {
			bag := p.bindProperties(node, schema.Properties, props)
			return ast.NewSchemaLiteral(node, schema, bag)
		}
	}

	return nil
}

// bindServiceRef binds a string to a service reference, resulting in a ServiceRef.  The reference is expected
// to be in the form "<service>[:<selector>]", where <service> is the name of a service that's currently in scope, and
// <selector> is an optional selector of a public service exported from that service.
func (p *binderValidatePhase) bindServiceRef(node *ast.Node, val string, ty *ast.Type) *ast.ServiceRef {
	glog.V(5).Infof("Binding capref '%v'", val)

	// Peel off the selector, if there is one.
	var sels string
	if selix := strings.LastIndex(val, ":"); selix != -1 {
		sels = val[selix+1:]
		val = val[:selix]
	}

	// Validate and convert the name and selector to names.
	var nm ast.Name
	if ast.IsName(val) {
		nm = ast.AsName(val)
	} else {
		p.Diag().Errorf(errors.ErrorNotAName.At(node), val)
	}
	var sel ast.Name
	if sels != "" {
		if ast.IsName(sels) {
			sel = ast.AsName(sels)
		} else {
			p.Diag().Errorf(errors.ErrorNotAName.At(node), sels)
		}
	}

	// If we have errors at this juncture, bail early, before it just gets worse.
	if !p.Diag().Success() {
		return nil
	}

	// Bind the name to a service.
	var ref *ast.ServiceRef
	if svc, ok := p.b.LookupService(ast.Name(nm)); ok {
		svct := svc.BoundType
		util.AssertMF(svct != nil, "Expected service '%v' to have a type", svc.Name)

		var selsvc *ast.Service
		if sel == "" {
			// If no selector was specified, just use the service itself as the selsvc.
			selsvc = svc
		} else if sel == "." {
			// A special dot selector can be used to pick the sole public service.
			if len(svct.Services.Public) == 0 {
				p.Diag().Errorf(errors.ErrorServiceHasNoPublics.At(node), svc.Name, svct.Name)
			} else if len(svct.Services.Public) == 1 {
				for _, pub := range svct.Services.Public {
					selsvc = pub
					break
				}
			} else {
				util.Assert(len(svct.Services.Public) > 1)
				p.Diag().Errorf(errors.ErrorServiceHasManyPublics.At(node), svc.Name, svct.Name)
			}
		} else {
			// If a selector was specified, ensure that it actually exists.
			if entry, ok := svct.Services.Public[sel]; ok {
				selsvc = entry
			} else {
				// The selector wasn't found.  Issue an error.  If there's a private service by that name,
				// say so, for better diagnostics.
				if _, has := svct.Services.Private[sel]; has {
					p.Diag().Errorf(errors.ErrorServiceSelectorIsPrivate.At(node), sel, svc.Name, svct.Name)
				} else {
					p.Diag().Errorf(errors.ErrorServiceSelectorNotFound.At(node), sel, svc.Name, svct.Name)
				}
			}
		}

		if selsvc != nil {
			// If there is an expected type, now ensure that the selected Service is of the right kind.
			util.Assert(selsvc.BoundType != nil)
			if ty != nil && !subclassOf(selsvc.BoundType, ty) {
				p.Diag().Errorf(errors.ErrorIncorrectType.At(node), ty, selsvc.BoundType.Name)
			}

			ref = &ast.ServiceRef{
				Name:     nm,
				Selector: sel,
				Service:  svc,
				Selected: selsvc,
			}
		}
	} else {
		p.Diag().Errorf(errors.ErrorServiceNotFound.At(node), nm)
	}

	return ref
}

// subclassOf checks that the left type ("typ") is equal to or a subclass of the right type ("or").  The right type is a
// union between *ast.Stack and *ast.UninstStack, so that it can be an uninstantiated type if needed.
func subclassOf(typ *ast.Stack, of *ast.Type) bool {
	for typ != nil {
		if typ == of.Stack {
			// If the type matches the target directly, obviously it's a hit.
			return true
		} else if of.UninstStack != nil {
			// If the type was produced from the same "document" (uninstantiated type), then it's also a hit.  Note that
			// due to template expansion, we need to walk the document hierarchy to see if there's a match.
			doc := typ.Doc
			for doc != nil {
				if doc == of.UninstStack.Doc {
					return true
				}
				doc = doc.Parent
			}
		}
		// Finally, if neither of those worked, we must see if there's a base class and keep searching.
		typ = typ.BoundBase
	}
	return false
}

// refToName converts a reference to its simple symbolic name.
func refToName(ref ast.Ref) ast.Name {
	return ref.MustParse().Name
}
