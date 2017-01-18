// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuIL symbol and token types.
package tokens

// Tokens.
type Token string   // a valid symbol token.
type Package Token  // a symbol token that resolves to a package.
type Module Token   // a symbol token that resolves to a module.
type Type Token     // a symbol token that resolves to a type.
type Variable Token // a symbol token that resolves to a variable.
type Function Token // a symbol token that resolves to a function.
