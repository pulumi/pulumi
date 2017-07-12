// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package symbols

import (
	"github.com/pulumi/lumi/pkg/compiler/ast"
)

// Function is an interface common to all functions.
type Function interface {
	Symbol
	Function() ast.Function
	Signature() *FunctionType
	SpecialModInit() bool // true if this is a module initializer.
}

var _ Symbol = (Function)(nil)
