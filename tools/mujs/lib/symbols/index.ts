// Copyright 2016 Marapongo, Inc. All rights reserved.

// Tokens.
export type Token = string;        // a valid symbol token.
export type ModuleToken = Token;   // a symbol token that resolves to a module.
export type TypeToken = Token;     // a symbol token that resolves to a type.
export type VariableToken = Token; // a symbol token that resolves to a variable.
export type FunctionToken = Token; // a symbol token that resolves to a function.

// Identifiers.
export type Identifier = string; // a valid identifier:  (letter|"_") (letter | digit | "_")*

