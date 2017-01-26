// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/tokens"
)

// Unwind instructs callers how to unwind the stack.
type Unwind struct {
	Break    bool         // true if breaking.
	Continue bool         // true if continuing.
	Label    *tokens.Name // a label being sought.
	Return   bool         // true if returning.
	Returned *Object      // an object being returned.
	Throw    bool         // true if raising an exception.
	Thrown   *Object      // an exception object being thrown.
}

func breakUnwind(label *tokens.Name) *Unwind    { return &Unwind{Break: true, Label: label} }
func continueUnwind(label *tokens.Name) *Unwind { return &Unwind{Continue: true, Label: label} }
func returnUnwind(ret *Object) *Unwind          { return &Unwind{Return: true, Returned: ret} }
func throwUnwind(thrown *Object) *Unwind        { return &Unwind{Throw: true, Thrown: thrown} }
