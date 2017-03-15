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
}

var _ Symbol = (Function)(nil)
