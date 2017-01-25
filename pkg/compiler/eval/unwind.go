// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/tokens"
)

// unwind instructs callers how to unwind the stack.
type unwind struct {
	Break    bool         // true if breaking.
	Continue bool         // true if continuing.
	Label    *tokens.Name // a label being sought.
	Return   bool         // true if returning.
	Returned *Object      // an object being returned.
	Throw    bool         // true if raising an exception.
	Thrown   *Object      // an exception object being thrown.
}

func breakUnwind(label *tokens.Name) *unwind    { return &unwind{Break: true, Label: label} }
func continueUnwind(label *tokens.Name) *unwind { return &unwind{Continue: true, Label: label} }
func returnUnwind(ret *Object) *unwind          { return &unwind{Return: true, Returned: ret} }
func throwUnwind(thrown *Object) *unwind        { return &unwind{Throw: true, Thrown: thrown} }
