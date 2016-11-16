// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// PTAnalyzer knows how to walk and validate parse trees.
type PTAnalyzer interface {
	// Diag fetches the diagnostics sink used by this analyzer.
	Diag() diag.Sink

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
	if stack.Name == "" {
		a.Diag().Errorf(errors.MissingStackName.WithDocument(doc))
	}
}
