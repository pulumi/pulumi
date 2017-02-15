// Copyright 2016 Marapongo, Inc. All rights reserved.

package rt

import (
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Unwind instructs callers how to unwind the stack.
type Unwind struct {
	kind      unwindKind   // the kind of the unwind.
	label     *tokens.Name // a label being sought (valid only on break/continue).
	returned  *Object      // an object being returned (valid only on return).
	exception *Exception   // information about the exception in flight (valid only on throw).
}

// Exception captures information about a thrown exception (object, source, and stack).
type Exception struct {
	Thrown *Object       // an exception object being thrown (valid only on throw).
	Node   diag.Diagable // the location that the throw occurred.
	Stack  *StackFrame   // the full linked stack trace.
}

// unwindKind is the kind of unwind being performed.
type unwindKind int

const (
	breakUnwind unwindKind = iota
	continueUnwind
	returnUnwind
	throwUnwind
)

func NewBreakUnwind(label *tokens.Name) *Unwind    { return &Unwind{kind: breakUnwind, label: label} }
func NewContinueUnwind(label *tokens.Name) *Unwind { return &Unwind{kind: continueUnwind, label: label} }
func NewReturnUnwind(ret *Object) *Unwind          { return &Unwind{kind: returnUnwind, returned: ret} }

func NewThrowUnwind(thrown *Object, node diag.Diagable, stack *StackFrame) *Unwind {
	contract.Require(node != nil, "node")
	contract.Require(stack != nil, "stack")
	return &Unwind{
		kind: throwUnwind,
		exception: &Exception{
			Thrown: thrown,
			Node:   node,
			Stack:  stack,
		},
	}
}

func (uw *Unwind) Break() bool    { return uw.kind == breakUnwind }
func (uw *Unwind) Continue() bool { return uw.kind == continueUnwind }
func (uw *Unwind) Return() bool   { return uw.kind == returnUnwind }
func (uw *Unwind) Throw() bool    { return uw.kind == throwUnwind }

func (uw *Unwind) Label() *tokens.Name {
	contract.Assert(uw.Break() || uw.Continue())
	return uw.label
}

func (uw *Unwind) Returned() *Object {
	contract.Assert(uw.Return())
	return uw.returned
}

func (uw *Unwind) Exception() *Exception {
	contract.Assert(uw.Throw())
	return uw.exception
}
