// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/util/contract"
)

// NewException produces a new exception in the evaluator using the current location and stack frames.
func (e *evaluator) NewException(node diag.Diagable, msg string, args ...interface{}) *rt.Object {
	contract.Require(node != nil, "node")
	return e.alloc.NewException(node, e.stack, msg, args...)
}

func (e *evaluator) NewNullObjectException(node diag.Diagable) *rt.Object {
	return e.NewException(node, "Target object is null")
}

func (e *evaluator) NewNegativeArrayLengthException(node diag.Diagable) *rt.Object {
	return e.NewException(node, "Invalid array size (must be >= 0)")
}

func (e *evaluator) NewIncorrectArrayElementCountException(node diag.Diagable, expect int, got int) *rt.Object {
	return e.NewException(node, "Invalid number of array elements; expected <=%v, got %v", expect, got)
}

func (e *evaluator) NewInvalidCastException(node diag.Diagable, from symbols.Type, to symbols.Type) *rt.Object {
	return e.NewException(node, "Cannot cast object of type '%v' to '%v'", from, to)
}
