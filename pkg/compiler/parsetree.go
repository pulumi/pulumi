// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
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

func (a *ptAnalyzer) VisitProperty(parent *ast.Stack, name string, param *ast.Property) {
	// Decorate the AST with contextual information so subsequent passes can operate context-free.
	param.Name = name
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
	bag map[string]interface{}) ast.Service {
	var typ ast.Name
	t, has := bag["type"]
	if has {
		// If the bag contains a type, ensure that it is a string.
		ts, ok := t.(string)
		if ok {
			typ = ast.Name(ts)
		} else {
			a.Diag().Errorf(errors.ErrorIllegalMufileSyntax.WithDocument(parent.Doc), "service type must be a string")
		}
	}

	return ast.Service{
		Name:   name,
		Type:   ast.Ref(typ),
		Public: public,
		Extra:  bag,
	}
}

func (a *ptAnalyzer) VisitService(pstack *ast.Stack, parent *ast.Services, name ast.Name, public bool,
	svc *ast.Service) {
}
