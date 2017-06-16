// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rt

import (
	"fmt"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
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
		if uw.exception != nil {
			return fmt.Sprintf("<throw: %v>", *uw.exception)
		}
		return fmt.Sprintf("<throw>")
	}
	contract.Failf("Unrecognized unwind kind")
	return ""
}
