// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
)

// Function is an interface common to all functions.
type Function interface {
	Symbol
	Function() ast.Function
	Signature() *FunctionType
}

var _ Symbol = (Function)(nil)
