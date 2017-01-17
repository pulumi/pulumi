// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuIL symbol and token types.
package symbols

// Tokens.
type Token string        // a valid symbol token.
type ModuleToken Token   // a symbol token that resolves to a module.
type TypeToken Token     // a symbol token that resolves to a type.
type VariableToken Token // a symbol token that resolves to a variable.
type FunctionToken Token // a symbol token that resolves to a function.

// Accessibility modifiers.
type Accessibility string // accessibility modifiers common to all.
const (
	PublicAccessibility  Accessibility = "public"
	PrivateAccessibility               = "private"
)

type ClassMemberAccessibility Accessibility // accessibility modifiers for class members.
const (
	PublicClassAccessibility    ClassMemberAccessibility = "public"
	PrivateClassAccessibility                            = "private"
	ProtectedClassAccessibility                          = "protected"
)
