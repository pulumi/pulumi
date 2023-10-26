// Copyright 2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package esc

import (
	"github.com/pulumi/esc/schema"
)

// An Expr holds information about an expression in an environment definition.
type Expr struct {
	// The range of the expression.
	Range Range `json:"range"`

	// The schema of the expression's result.
	Schema *schema.Schema `json:"schema,omitempty"`

	// The expression that defined this expression's base value, if any.
	Base *Expr `json:"base,omitempty"`

	// Ranges for the object's keys, if this is an object expression.
	KeyRanges map[string]Range `json:"keyRanges,omitempty"`

	// The fields below act as a discriminated union. Only one must be non-nil at any given time. If all fields are nil,
	// then this Expr is a null literal expression.

	// The literal value, if this is a literal expression (nil, bool, json.Number, or string)
	Literal any `json:"literal,omitempty"`

	// The interpolations, if this is a string interpolation expression.
	Interpolate []Interpolation `json:"interpolate,omitempty"`

	// The property accessors, if this is a symbol expression.
	Symbol []PropertyAccessor `json:"symbol,omitempty"`

	// The access, if this is an access expression.
	Access *AccessExpr `json:"access,omitempty"`

	// The list elements, if this is a list expression.
	List []Expr `json:"list,omitempty"`

	// The object properties, if this is an object expression.
	Object map[string]Expr `json:"object,omitempty"`

	// The builtin, if this is a call to a builtin function.
	Builtin *BuiltinExpr `json:"builtin,omitempty"`
}

// An Interpolation holds information about a part of an interpolated string expression.
type Interpolation struct {
	// The text of the expression. Precedes the stringified Value in the output.
	Text string `json:"text,omitempty"`

	// The value to interpolate.
	Value []PropertyAccessor `json:"value,omitempty"`
}

// An Accessor is an element index or property name.
type Accessor struct {
	// The integer index of the element to access. Mutually exclusive with Key.
	Index *int `json:"index,omitempty"`

	// The key of the property to access. Mutually exclusive with Index.
	Key *string `json:"key,omitempty"`
}

// A PropertyAccessor is a single accessor that is associated with a resolved value.
type PropertyAccessor struct {
	Accessor

	// The range of the expression that defines the resolved value.
	Value Range `json:"value"`
}

// An AccessExpr represents a property access with a receiving value.
type AccessExpr struct {
	// The receiver to access.
	Receiver Range `json:"receiver"`

	// The accessors to evaluate.
	Accessors []Accessor `json:"accessors"`
}

// A BuiltinExpr is a call to a builtin function.
type BuiltinExpr struct {
	Name      string         `json:"name"`
	NameRange Range          `json:"nameRange"`
	ArgSchema *schema.Schema `json:"argSchema"`
	Arg       Expr           `json:"arg"`
}

// A Range defines a range within an environment definition.
type Range struct {
	// The name of the environment.
	Environment string `json:"environment,omitempty"`

	// The beginning of the range.
	Begin Pos `json:"begin"`

	// The end of the range.
	End Pos `json:"end"`
}

// Contains returns true if the range contains the given position.
func (r Range) Contains(pos Pos) bool {
	if pos.Byte >= r.Begin.Byte && pos.Byte < r.End.Byte {
		return true
	}
	if pos.Line < r.Begin.Line || pos.Line > r.End.Line {
		return false
	}
	if r.Begin.Line == r.End.Line {
		return pos.Line == r.Begin.Line && pos.Column >= r.Begin.Column && pos.Column < r.End.Column
	}
	return true
}

// A Pos defines a position within an environment definition.
type Pos struct {
	// Line is the source code line where this position points. Lines are counted starting at 1 and incremented for each
	// newline character encountered.
	Line int `json:"line"`

	// Column is the source code column where this position points. Columns are counted in visual cells starting at 1,
	// and are incremented roughly per grapheme cluster encountered.
	Column int `json:"column"`

	// Byte is the byte offset into the file where the indicated position begins.
	Byte int `json:"byte"`
}
