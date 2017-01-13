// Copyright 2016 Marapongo, Inc. All rights reserved.

// Tokens.
export type Token = string;        // a valid symbol token.
export type ModuleToken = Token;   // a symbol token that resolves to a module.
export type TypeToken = Token;     // a symbol token that resolves to a type.
export type VariableToken = Token; // a symbol token that resolves to a variable.
export type FunctionToken = Token; // a symbol token that resolves to a function.

export const tokenSep = "/";       // the separator for token "parts" (modules names, etc).

// Accessibility modifiers.
export type Accessibility            = "public" | "private";        // accessibility modifiers common to all.
export type ClassMemberAccessibility = Accessibility | "protected"; // accessibility modifiers for class members.

// Accessibility modifier constants.
export const publicAccessibility: Accessibility               = "public";
export const privateAccessibility: Accessibility              = "private";
export const protectedAccessibility: ClassMemberAccessibility = "protected";

// Special variable tokens.
export const specialVariableThis: VariableToken  = ".this";  // the current object (for class methods).
export const specialVariableSuper: VariableToken = ".super"; // the parent class object (for class methods).

// Special function tokens.
export const specialFunctionEntryPoint: FunctionToken  = ".main"; // the special package entrypoint function.
export const specialFunctionInitializer: FunctionToken = ".init"; // the special module/class initialize function.
export const specialFunctionConstructor: FunctionToken = ".ctor"; // the special class instance constructor function.

