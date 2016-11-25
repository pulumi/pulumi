// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/workspace"
)

// buildDocumentFE runs the front-end phases of the compiler.
func (c *compiler) buildDocumentFE(w workspace.W, doc *diag.Document) *ast.Stack {
	// If there's a workspace-wide settings file available, load it up.
	wdoc, err := w.ReadSettings()
	if err != nil {
		// TODO: we should include the file information in the error message.
		c.Diag().Errorf(errors.ErrorIO, err)
		return nil
	}

	// Now create a parser to create ASTs from the workspace settings file and Mufile.
	p := NewParser(c)
	if wdoc != nil {
		// Store the parsed AST on the workspace object itself.
		*w.Settings() = *p.ParseWorkspace(doc)
	}

	// TODO[marapongo/mu#7]: for top-level stacks with arguments, they need to be supplied at the CLI.
	stack := p.ParseStack(doc, nil)

	// If any parser errors occurred, bail now to prevent needlessly obtuse error messages.
	if !p.Diag().Success() {
		return nil
	}

	// Now create a parse tree analyzer to walk the parse trees and ensure that all is well.
	ptAnalyzer := NewPTAnalyzer(c)
	if wdoc != nil {
		ptAnalyzer.AnalyzeWorkspace(w.Settings())
	}
	ptAnalyzer.AnalyzeStack(stack)

	// If any errors happened during parse tree analysis, exit.
	if !p.Diag().Success() {
		return nil
	}

	return stack
}
