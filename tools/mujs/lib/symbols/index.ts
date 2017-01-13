// Copyright 2016 Marapongo, Inc. All rights reserved.

// Tokens.
export type Token = string;        // a valid symbol token.
export type ModuleToken = Token;   // a symbol token that resolves to a module.
export type TypeToken = Token;     // a symbol token that resolves to a type.
export type VariableToken = Token; // a symbol token that resolves to a variable.
export type FunctionToken = Token; // a symbol token that resolves to a function.

export const moduleSep = ":";               // a character delimiting module / member names (e.g., "module:member").
export const selfModule: ModuleToken = "."; // a self-referential token for the current module.

// Accessibility modifiers.
export type Accessibility            = "public" | "private";        // accessibility modifiers common to all.
export type ClassMemberAccessibility = Accessibility | "protected"; // accessibility modifiers for class members.

// Accessibility modifier constants.
export const publicAccessibility: Accessibility               = "public";
export const privateAccessibility: Accessibility              = "private";
export const protectedAccessibility: ClassMemberAccessibility = "protected";

// Special variable tokens.
export const thisVariable: VariableToken  = ".this";  // the current object (for class methods).
export const superVariable: VariableToken = ".super"; // the parent class object (for class methods).

// Special function tokens.
export const entryPointFunction: FunctionToken  = ".main"; // the special package entrypoint function.
export const initializerFunction: FunctionToken = ".init"; // the special module/class initialize function.
export const constructorFunction: FunctionToken = ".ctor"; // the special class instance constructor function.

// Special type tokens.
export const anyType: TypeToken    = "any";
export const stringType: TypeToken = "string";
export const numberType: TypeToken = "number";
export const boolType: TypeToken   = "bool";

