// Copyright 2017 Pulumi, Inc. All rights reserved.

package eval

import (
	"fmt"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// NewException produces a new exception in the evaluator using the current location and stack frames.
func (e *evaluator) NewException(node diag.Diagable, msg string, args ...interface{}) *rt.Unwind {
	contract.Require(node != nil, "node")
	thrown := e.alloc.NewString(node, fmt.Sprintf(msg, args...))
	return rt.NewThrowUnwind(thrown, node, e.stack)
}

func (e *evaluator) NewNullObjectException(node diag.Diagable) *rt.Unwind {
	return e.NewException(node, "Target object is null")
}

func (e *evaluator) NewNameNotDefinedException(node diag.Diagable, name tokens.Name) *rt.Unwind {
	return e.NewException(node, "Name '%v' is not defined", name)
}

func (e *evaluator) NewNegativeArrayLengthException(node diag.Diagable) *rt.Unwind {
	return e.NewException(node, "Invalid array size (must be >= 0)")
}

func (e *evaluator) NewIncorrectArrayElementCountException(node diag.Diagable, expect int, got int) *rt.Unwind {
	return e.NewException(node, "Invalid number of array elements; expected <=%v, got %v", expect, got)
}

func (e *evaluator) NewInvalidCastException(node diag.Diagable, from symbols.Type, to symbols.Type) *rt.Unwind {
	return e.NewException(node, "Cannot cast object of type '%v' to '%v'", from, to)
}

func (e *evaluator) NewIllegalInvokeTargetException(node diag.Diagable, target symbols.Type) *rt.Unwind {
	return e.NewException(node, "Expected a function as the target of an invoke; got '%v'", target)
}
