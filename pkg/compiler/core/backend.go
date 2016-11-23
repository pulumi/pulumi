// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/ast"
)

// Backend represents the last phase during compilation; it handles code-generation and emission.
type Backend interface {
	Phase
	// CodeGen lowers and emits code for the given target and stack.
	CodeGen(comp Compiland)
}

// Compiland contains all of settings passed from front-end to back-end compiler phases.
type Compiland struct {
	Cluster *ast.Cluster // the target cluster.
	Stack   *ast.Stack   // the root stack to compile.
}
