// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/eval/rt"
)

func (e *evaluator) NewNullObjectException() *rt.Object {
	return e.alloc.NewException("Target object is null")
}

func (e *evaluator) NewNegativeArrayLengthException() *rt.Object {
	return e.alloc.NewException("Invalid array size (must be >= 0)")
}

func (e *evaluator) NewIncorrectArrayElementCountException(expect int, got int) *rt.Object {
	return e.alloc.NewException("Invalid number of array elements; expected <=%v, got %v", expect, got)
}
