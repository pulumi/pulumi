// Copyright 2017 Pulumi, Inc. All rights reserved.

package symbols

import (
	"github.com/pulumi/coconut/pkg/compiler/ast"
)

// Function is an interface common to all functions.
type Function interface {
	Symbol
	Function() ast.Function
	Signature() *FunctionType
	SpecialModInit() bool // true if this is a module initializer.
}

var _ Symbol = (Function)(nil)
