// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package rt

import (
	"fmt"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/types"
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// Unwind instructs callers how to unwind the stack.
type Unwind struct {
	kind     unwindKind   // the kind of the unwind.
	label    *tokens.Name // a label being sought (valid only on break/continue).
	returned *Object      // an object being returned (valid only on return).
	thrown   *Exception   // information about the exception in flight (valid only on throw).
}

// Exception captures information about a thrown exception (object, source, and stack).
type Exception struct {
	Obj   *Object       // an exception object being thrown (valid only on throw).
	Node  diag.Diagable // the location that the throw occurred.
	Stack *StackFrame   // the full linked stack trace.
}

// Message returns a prettified message for the exception object, including its string and stack trace.
func (ex *Exception) Message(d diag.Sink) string {
	if ex == nil {
		return "no details available"
	}

	var msg string
	if ex.Obj.Type() == types.String {
		msg = ex.Obj.StringValue() // use the basic string value.
	} else {
		msg = "\n" + ex.Obj.Details(false, "\t") // convert the thrown object into a detailed string
	}
	return msg + "\n" + ex.Stack.Trace(d, "\t", ex.Node)
}

// unwindKind is the kind of unwind being performed.
type unwindKind int

const (
	breakUnwind unwindKind = iota
	cancelUnwind
	continueUnwind
	returnUnwind
	throwUnwind
)

func NewBreakUnwind(label *tokens.Name) *Unwind    { return &Unwind{kind: breakUnwind, label: label} }
func NewCancelUnwind() *Unwind                     { return &Unwind{kind: cancelUnwind} }
func NewContinueUnwind(label *tokens.Name) *Unwind { return &Unwind{kind: continueUnwind, label: label} }
func NewReturnUnwind(ret *Object) *Unwind          { return &Unwind{kind: returnUnwind, returned: ret} }

func NewThrowUnwind(obj *Object, node diag.Diagable, stack *StackFrame) *Unwind {
	contract.Require(node != nil, "node")
	contract.Require(stack != nil, "stack")
	return &Unwind{
		kind: throwUnwind,
		thrown: &Exception{
			Obj:   obj,
			Node:  node,
			Stack: stack,
		},
	}
}

func (uw *Unwind) Break() bool    { return uw.kind == breakUnwind }
func (uw *Unwind) Cancel() bool   { return uw.kind == cancelUnwind }
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

func (uw *Unwind) Thrown() *Exception {
	contract.Assert(uw.Throw())
	return uw.thrown
}

func (uw *Unwind) String() string {
	if uw.Break() {
		if uw.label != nil {
			return fmt.Sprintf("<break: %v>", *uw.label)
		}
		return fmt.Sprintf("<break>")
	} else if uw.Continue() {
		if uw.label != nil {
			return fmt.Sprintf("<continue: %v>", *uw.label)
		}
		return fmt.Sprintf("<continue>")
	} else if uw.Return() {
		if uw.returned != nil {
			return fmt.Sprintf("<return: %v>", *uw.returned)
		}
		return fmt.Sprintf("<return>")
	} else if uw.Throw() {
		if uw.thrown != nil {
			return fmt.Sprintf("<throw: %v>", *uw.thrown)
		}
		return fmt.Sprintf("<throw>")
	}
	contract.Failf("Unrecognized unwind kind")
	return ""
}
