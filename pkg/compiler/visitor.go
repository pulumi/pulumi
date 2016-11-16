// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

// Visitor unifies all visitation patterns under a single interface.
type Visitor interface {
	Phase
	VisitMetadata(doc *diag.Document, kind string, meta *ast.Metadata)
	VisitStack(doc *diag.Document, stack *ast.Stack)
	VisitParameter(doc *diag.Document, name string, param *ast.Parameter)
	VisitService(doc *diag.Document, name string, public bool, svc *ast.Service)
	VisitDependency(doc *diag.Document, name string, dep *ast.Dependency)
}
