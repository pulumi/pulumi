// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Tokens.
export type Token = string;            // a valid symbol token.
export type PackageToken = Token;      // a symbol token that resolves to a package.
export type ModuleToken = Token;       // a symbol token that resolves to a module.
export type ModuleMemberToken = Token; // a symbol token that resolves to a module member.
export type ClassMemberToken = Token;  // a symbol token that resolves to a class member.
export type TypeToken = Token;         // a symbol token that resolves to a type.
export type VariableToken = Token;     // a symbol token that resolves to a variable.
export type FunctionToken = Token;     // a symbol token that resolves to a function.

export const tokenDelimiter = ":";     // the character delimiting modules/members/etc.

// Names.
export type Name = string;             // a valid identifier name.

