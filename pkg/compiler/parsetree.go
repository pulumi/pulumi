// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"strings"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// PTAnalyzer knows how to walk and validate parse trees.
type PTAnalyzer interface {
	core.Visitor

	// AnalyzeStack checks the validity of an entire Stack parse tree.
	AnalyzeStack(stack *ast.Stack)
	// AnalyzeWorkspace checks the validity of an entire Workspace parse tree.
	AnalyzeWorkspace(w *ast.Workspace)
}

// NewPTAnalayzer allocates a new PTAnalyzer associated with the given Compiler.
func NewPTAnalyzer(c Compiler) PTAnalyzer {
	return &ptAnalyzer{c: c}
}

type ptAnalyzer struct {
	c Compiler
}

var _ PTAnalyzer = &ptAnalyzer{} // compile-time assert that binder implements PTAnalyzer.

func (a *ptAnalyzer) Diag() diag.Sink {
	return a.c.Diag()
}

func (a *ptAnalyzer) AnalyzeStack(stack *ast.Stack) {
	glog.Infof("Parsetree analyzing Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsetree analysis for Mu Stack %v completed w/ %v warnings and %v errors",
				stack.Name, a.Diag().Warnings(), a.Diag().Errors())
		}()
	}

	// Use an InOrderVisitor to walk the tree in-order; this handles determinism for us.
	v := core.NewInOrderVisitor(a, nil)
	v.VisitStack(stack)
}

func (a *ptAnalyzer) AnalyzeWorkspace(w *ast.Workspace) {
	glog.Infof("Parsetree analyzing workspace file: %v", w.Doc.File)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsetree analysis for workspace %v completed w/ %v warnings and %v errors",
				w.Doc.File, a.Diag().Warnings(), a.Diag().Errors())
		}()
	}

	// Use an InOrderVisitor to walk the tree in-order; this handles determinism for us.
	v := core.NewInOrderVisitor(a, nil)
	v.VisitWorkspace(w)
}

func (a *ptAnalyzer) VisitWorkspace(w *ast.Workspace) {
}

func (a *ptAnalyzer) VisitCluster(name string, cluster *ast.Cluster) {
	// Decorate the AST with contextual information so subsequent passes can operate context-free.
	cluster.Name = name
}

func (a *ptAnalyzer) VisitDependency(parent *ast.Workspace, ref ast.Ref, dep *ast.Dependency) {
}

func (a *ptAnalyzer) VisitStack(stack *ast.Stack) {
}

func (a *ptAnalyzer) VisitSchemas(parent *ast.Stack, schemas *ast.Schemas) {
}

func (a *ptAnalyzer) VisitSchema(pstack *ast.Stack, parent *ast.Schemas, name ast.Name, public bool,
	schema *ast.Schema) {
	// Decorate the AST with contextual information.
	schema.Name = name
	schema.Public = public

	// If the schema has a base type listed, parse it to the best of our ability.
	if schema.Base != "" {
		schema.BoundBase = a.parseType(schema.Base)
	}
}

func (a *ptAnalyzer) VisitProperty(parent *ast.Stack, schema *ast.Schema, name string, prop *ast.Property) {
	// Decorate the AST with contextual information so subsequent passes can operate context-free.
	prop.Name = name

	// Parse the property type to the best of our ability at this phase in the compiler.
	prop.BoundType = a.parseType(prop.Type)
}

func (a *ptAnalyzer) VisitServices(parent *ast.Stack, svcs *ast.Services) {
	// We need to expand the UntypedServiceMaps into strongly typed ServiceMaps.  As part of this, we also decorate the
	// AST with extra contextual information so subsequent passes can operate context-free.
	// TODO[marapongo/mu#4]: once we harden the marshalers, we should be able to largely eliminate this.
	svcs.Public = make(ast.ServiceMap)
	for _, name := range ast.StableUntypedServices(svcs.PublicUntyped) {
		svcs.Public[name] = a.untypedServiceToTyped(parent, name, true, svcs.PublicUntyped[name])
	}
	svcs.Private = make(ast.ServiceMap)
	for _, name := range ast.StableUntypedServices(svcs.PrivateUntyped) {
		svcs.Private[name] = a.untypedServiceToTyped(parent, name, false, svcs.PrivateUntyped[name])
	}
}

func (a *ptAnalyzer) untypedServiceToTyped(parent *ast.Stack, name ast.Name, public bool,
	bag map[string]interface{}) *ast.Service {
	var typ ast.Name
	t, has := bag["type"]
	if has {
		// If the bag contains a type, ensure that it is a string.
		ts, ok := t.(string)
		if ok {
			typ = ast.Name(ts)
		} else {
			a.Diag().Errorf(errors.ErrorIllegalMufileSyntax.At(parent), "service type must be a string")
		}

		// Delete the type property so it's not considered semantically meaningful for the target.
		delete(bag, "type")
	}

	return &ast.Service{
		Name:       name,
		Type:       ast.Ref(typ),
		Public:     public,
		Properties: bag,
	}
}

func (a *ptAnalyzer) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
}

// parseType produces an ast.Type.  This will not have been bound yet, so for example, we won't know whether
// an arbitrary non-primitive reference name references a stack or a schema, however at least this is a start.
func (a *ptAnalyzer) parseType(ref ast.Ref) *ast.Type {
	refs := string(ref)

	mix := strings.Index(refs, ast.TypeDecorsMapPrefix)
	if mix == 0 {
		// If we have a map, find the separator, and then parse the key and value parts.
		rest := refs[mix+len(ast.TypeDecorsMapPrefix):]
		if sep := strings.Index(rest, ast.TypeDecorsMapSeparator); sep != -1 {
			keyn := ast.Ref(rest[:sep])
			valn := ast.Ref(rest[:sep+len(ast.TypeDecorsMapSeparator)])
			keyt := a.parseType(keyn)
			valt := a.parseType(valn)
			if keyt != nil && valt != nil {
				return ast.NewMapType(keyt, valt)
			}
		} else {
			a.Diag().Errorf(errors.ErrorIllegalMapLikeSyntax, refs)
		}
	} else if aix := strings.Index(refs, ast.TypeDecorsArrayPrefix); aix != -1 {
		if aix == 0 {
			// If we have an array, peel off the front and keep going.
			rest := refs[aix+len(ast.TypeDecorsArrayPrefix):]
			if elem := a.parseType(ast.Ref(rest)); elem != nil {
				return ast.NewArrayType(elem)
			}
		} else {
			// The array part was in the wrong position.  Issue an error.  Maybe they did T[] instead of []T?
			a.Diag().Errorf(errors.ErrorIllegalArrayLikeSyntax, refs)
		}
	} else if mix != -1 {
		// The map part was in the wrong position.  Issue an error.
		a.Diag().Errorf(errors.ErrorIllegalMapLikeSyntax, refs)
	} else {
		// Otherwise, there are no decorators.  Parse the result as either a primitive type or unresolved name.
		switch ast.PrimitiveType(refs) {
		case ast.PrimitiveTypeAny:
			return ast.NewAnyType()
		case ast.PrimitiveTypeString:
			return ast.NewStringType()
		case ast.PrimitiveTypeNumber:
			return ast.NewNumberType()
		case ast.PrimitiveTypeBool:
			return ast.NewBoolType()
		case ast.PrimitiveTypeService:
			return ast.NewServiceType()
		}

		// If we didn't recognize anything thus far, it's a simple name.  We don't yet know what it references --
		// it could be a stack, schema, or even a completely bogus, missing name -- so just store it as it is.
		if _, err := ref.Parse(); err != nil {
			a.Diag().Errorf(errors.ErrorIllegalNameLikeSyntax, refs, err)
		} else {
			return ast.NewUnresolvedRefType(&ref)
		}
	}

	return nil
}
