// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
)

// Backend represents the last phase during compilation; it handles code-generation and emission.
type Backend interface {
	Phase
	// CodeGen lowers and emits code for the given target and stack.
	CodeGen(comp Compiland)
}

// Compiland contains all of settings passed from front-end to back-end compiler phases.
type Compiland struct {
	Target *ast.Target    // the compilation target.
	Doc    *diag.Document // the document from which the root stack came.
	Stack  *ast.Stack     // the root stack to compile.
}
