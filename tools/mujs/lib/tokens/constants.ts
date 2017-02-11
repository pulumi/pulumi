// Copyright 2016 Marapongo, Inc. All rights reserved.

import {FunctionToken, ModuleToken, TypeToken, VariableToken} from "./tokens";

// Accessibility modifiers.
export type Accessibility            = "public" | "private";        // accessibility modifiers common to all.
export type ClassMemberAccessibility = Accessibility | "protected"; // accessibility modifiers for class members.

// Accessibility modifier constants.
export const publicAccessibility: Accessibility               = "public";
export const privateAccessibility: Accessibility              = "private";
export const protectedAccessibility: ClassMemberAccessibility = "protected";

// Special module tokens.
export const selfModule: ModuleToken = "."; // a self-referential token for the current module.

// Special variable tokens.
export const thisVariable: VariableToken  = ".this";  // the current object (for class methods).
export const superVariable: VariableToken = ".super"; // the parent class object (for class methods).

// Special function tokens.
export const entryPointFunction: FunctionToken  = ".main"; // the special package entrypoint function.
export const initializerFunction: FunctionToken = ".init"; // the special module/class initialize function.
export const constructorFunction: FunctionToken = ".ctor"; // the special class instance constructor function.

// Special type tokens.
export const objectType: TypeToken  = "object";
export const stringType: TypeToken  = "string";
export const numberType: TypeToken  = "number";
export const boolType: TypeToken    = "bool";
export const dynamicType: TypeToken = "dynamic";

