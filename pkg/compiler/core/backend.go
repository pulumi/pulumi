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
	CodeGen(target *ast.Target, doc *diag.Document, stack *ast.Stack)
}
