// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/blang/semver"
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// PTAnalyzer knows how to walk and validate parse trees.
type PTAnalyzer interface {
	core.Visitor

	// Analyze checks the validity of an entire parse tree (starting with a top-level Stack).
	Analyze(doc *diag.Document, stack *ast.Stack)
}

// NewPTAnalayzer allocates a new PTAnalyzer associated with the given Compiler.
func NewPTAnalyzer(c Compiler) PTAnalyzer {
	return &ptAnalyzer{c: c}
}

type ptAnalyzer struct {
	c Compiler
}

func (a *ptAnalyzer) Diag() diag.Sink {
	return a.c.Diag()
}

func (a *ptAnalyzer) Analyze(doc *diag.Document, stack *ast.Stack) {
	glog.Infof("Parsetree analyzing Mu Stack: %v", stack.Name)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsetree analysis for Mu Stack %v completed w/ %v warnings and %v errors",
				stack.Name, a.Diag().Warnings(), a.Diag().Errors())
		}()
	}

	// Use an InOrderVisitor to walk the tree in-order; this handles determinism for us.
	v := core.NewInOrderVisitor(a, nil)
	v.VisitStack(doc, stack)
}

func (a *ptAnalyzer) VisitStack(doc *diag.Document, stack *ast.Stack) {
	// Stack names are required.
	if stack.Name == "" {
		a.Diag().Errorf(errors.MissingStackName.WithDocument(doc))
	}

	// Stack versions must be valid semantic versions (and specifically, not ranges).  In other words, we need
	// a concrete version number like "1.3.9-beta2" and *not* a range like ">1.3.9".
	// TODO: should we require a version number?
	if stack.Version != "" {
		if _, err := semver.Parse(string(stack.Version)); err != nil {
			a.Diag().Errorf(errors.IllegalStackSemVer.WithDocument(doc), stack.Version)
		}
	}
}

func (a *ptAnalyzer) VisitCluster(doc *diag.Document, name string, cluster *ast.Cluster) {
	// Decorate the AST with contextual information so subsequent passes can operate context-free.
	cluster.Name = name
}

func (a *ptAnalyzer) VisitProperty(doc *diag.Document, name string, param *ast.Property) {
	// Decorate the AST with contextual information so subsequent passes can operate context-free.
	param.Name = name
}

func (a *ptAnalyzer) VisitDependency(doc *diag.Document, ref ast.Ref, dep *ast.Dependency) {
	// Dependency versions must be valid semantic versions *or* ranges.
	// TODO: should we require dependencies to have versions?
	ver := *dep
	if ver != "" {
		if _, err := semver.ParseRange(string(ver)); err != nil {
			a.Diag().Errorf(errors.IllegalDependencySemVer.WithDocument(doc), ref, ver)
		}
	}
}

func (a *ptAnalyzer) VisitBoundDependency(doc *diag.Document, ref ast.Ref, dep *ast.BoundDependency) {
}

func (a *ptAnalyzer) VisitServices(doc *diag.Document, svcs *ast.Services) {
	// We need to expand the UntypedServiceMaps into strongly typed ServiceMaps.  As part of this, we also decorate the
	// AST with extra contextual information so subsequent passes can operate context-free.
	// TODO[marapongo/mu#4]: once we harden the marshalers, we should be able to largely eliminate this.
	svcs.Public = make(ast.ServiceMap)
	for _, name := range ast.StableUntypedServices(svcs.PublicUntyped) {
		svcs.Public[name] = a.untypedServiceToTyped(doc, name, true, svcs.PublicUntyped[name])
	}
	svcs.Private = make(ast.ServiceMap)
	for _, name := range ast.StableUntypedServices(svcs.PrivateUntyped) {
		svcs.Private[name] = a.untypedServiceToTyped(doc, name, false, svcs.PrivateUntyped[name])
	}
}

func (a *ptAnalyzer) untypedServiceToTyped(doc *diag.Document,
	name ast.Name, public bool, bag map[string]interface{}) ast.Service {
	var typ ast.Name
	t, has := bag["type"]
	if has {
		// If the bag contains a type, ensure that it is a string.
		ts, ok := t.(string)
		if ok {
			typ = ast.Name(ts)
		} else {
			a.Diag().Errorf(errors.IllegalMufileSyntax.WithDocument(doc), "service type must be a string")
		}
	}

	return ast.Service{
		Name:   name,
		Type:   typ,
		Public: public,
		Extra:  bag,
	}
}

func (a *ptAnalyzer) VisitService(doc *diag.Document, name ast.Name, public bool, svc *ast.Service) {
}
