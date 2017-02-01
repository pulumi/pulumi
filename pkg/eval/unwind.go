// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Unwind instructs callers how to unwind the stack.
type Unwind struct {
	kind     unwindKind   // the kind of the unwind.
	label    *tokens.Name // a label being sought (valid only on break/continue).
	returned *rt.Object   // an object being returned (valid only on return).
	thrown   *rt.Object   // an exception object being thrown (valid only on throw).
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
func NewReturnUnwind(ret *rt.Object) *Unwind       { return &Unwind{kind: returnUnwind, returned: ret} }
func NewThrowUnwind(thrown *rt.Object) *Unwind     { return &Unwind{kind: throwUnwind, thrown: thrown} }

func (uw *Unwind) Break() bool    { return uw.kind == breakUnwind }
func (uw *Unwind) Continue() bool { return uw.kind == continueUnwind }
func (uw *Unwind) Return() bool   { return uw.kind == returnUnwind }
func (uw *Unwind) Throw() bool    { return uw.kind == throwUnwind }

func (uw *Unwind) Label() *tokens.Name {
	contract.Assert(uw.Break() || uw.Continue())
	return uw.label
}

func (uw *Unwind) Returned() *rt.Object {
	contract.Assert(uw.Return())
	return uw.returned
}

func (uw *Unwind) Thrown() *rt.Object {
	contract.Assert(uw.Throw())
	return uw.thrown
}
