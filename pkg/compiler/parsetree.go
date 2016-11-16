// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/blang/semver"
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// PTAnalyzer knows how to walk and validate parse trees.
type PTAnalyzer interface {
	Visitor

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
	a.VisitStack(doc, stack)
}

func (a *ptAnalyzer) VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata) {
	// Metadata names are required.
	if meta.Name == "" {
		a.Diag().Errorf(errors.MissingMetadataName.WithDocument(doc), kind)
	}

	// Metadata versions must be valid semantic versions (and specifically, not ranges).  In other words, we need
	// a concrete version number like "1.3.9-beta2" and *not* a range like ">1.3.9".
	// TODO: should we require a version number?
	if meta.Version != "" {
		if _, err := semver.Parse(string(meta.Version)); err != nil {
			a.Diag().Errorf(errors.IllegalMetadataSemVer.WithDocument(doc), kind, meta.Version)
		}
	}
}

func (a *ptAnalyzer) VisitStack(doc *diag.Document, stack *ast.Stack) {
	a.VisitMetadata(doc, "Stack", &stack.Metadata)
	for name, param := range stack.Parameters {
		a.VisitParameter(doc, name, param)
	}
	for name, svc := range stack.Public {
		a.VisitService(doc, name, true, svc)
	}
	for name, svc := range stack.Private {
		a.VisitService(doc, name, false, svc)
	}
	for name, dep := range stack.Dependencies {
		a.VisitDependency(doc, name, &dep)
	}
}

func (a *ptAnalyzer) VisitParameter(doc *diag.Document, name string, param *ast.Parameter) {
}

func (a *ptAnalyzer) VisitService(doc *diag.Document, name string, public bool, svc *ast.Service) {
}

func (a *ptAnalyzer) VisitDependency(doc *diag.Document, name string, dep *ast.Dependency) {
	// Dependency versions must be valid semantic versions *or* ranges.
	// TODO: should we require dependencies to have versions?
	ver := *dep
	if ver != "" {
		if _, err := semver.ParseRange(string(ver)); err != nil {
			a.Diag().Errorf(errors.IllegalDependencySemVer.WithDocument(doc), name, ver)
		}
	}
}
