// Copyright 2016 Marapongo, Inc. All rights reserved.

package types

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
)

// All of the primitive types.
var (
	Any    = symbols.NewPrimitiveType("any")
	Bool   = symbols.NewPrimitiveType("bool")
	Number = symbols.NewPrimitiveType("number")
	String = symbols.NewPrimitiveType("string")
	Null   = symbols.NewPrimitiveType("null")
	Error  = symbols.NewPrimitiveType("error")
)

// Primitives contains a map of all primitive types, keyed by their token/name.
var Primitives = map[tokens.TypeName]symbols.Type{
	Any.Nm:    Any,
	Bool.Nm:   Bool,
	Number.Nm: Number,
	String.Nm: String,
	Null.Nm:   Null,
	Error.Nm:  Error,
}

// Common weakly typed types.
var (
	AnyArray = symbols.NewArrayType(Any)
	AnyMap   = symbols.NewMapType(Any, Any)
)
