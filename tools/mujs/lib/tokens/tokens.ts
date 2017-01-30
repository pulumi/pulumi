// Copyright 2016 Marapongo, Inc. All rights reserved.

// Tokens.
export type Token = string;            // a valid symbol token.
export type PackageToken = Token;      // a symbol token that resolves to a package.
export type ModuleToken = Token;       // a symbol token that resolves to a module.
export type ModuleMemberToken = Token; // a symbol token that resolves to a module member.
export type ClassMemberToken = Token;  // a symbol token that resolves to a class member.
export type TypeToken = Token;         // a symbol token that resolves to a type.
export type VariableToken = Token;     // a symbol token that resolves to a variable.
export type FunctionToken = Token;     // a symbol token that resolves to a function.

export const moduleDelimiter = ":";       // the character following a package (before a module).
export const moduleMemberDelimiter = "/"; // the character following a module (before a module member).
export const classMemberDelimiter = ".";  // the character following a class name (before a class member).

