// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as il from "../il";
import * as symbols from "../symbols";

// TODO(joe): consider refactoring modifiers from booleans to enums.

// A top-level package definition.
export interface Package {
    name: string;         // a required fully qualified name.
    description?: string; // an optional informational description.
    author?: string;      // an optional author.
    website?: string;     // an optional website for additional information.
    license?: string;     // an optional license governing this package's usage.

    dependencies?: symbols.ModuleToken[];

    definitions?: Definitions;
}

// A set of module definitions; every package has an implicit top-level one, as does each submodule.
export interface Definitions {
    modules?:   Modules;   // submodules.
    variables?: Variables; // module-scoped variables.
    functions?: Functions; // module-scoped functions.
    classes?:   Classes;   // classes.
}

export type Modules =   Map<symbols.Identifier, Module>;   // a map of modules keyed by unique identifier.
export type Variables = Map<symbols.Identifier, Variable>; // a map of variables keyed by unique identifier.
export type Functions = Map<symbols.Identifier, Function>; // a map of functions keyed by unique identifier.
export type Classes =   Map<symbols.Identifier, Class>;    // a map of classes keyed by unique identifier.

export type Accessibility = "public" | "private";                          // accessibility modifiers common to all.
export type ClassMemberAccessibility = "public" | "private" | "protected"; // accessibility modifiers for class members.

export const specialFunctionEntryPoint =  ".main"; // the special package entrypoint function.
export const specialFunctionInitializer = ".init"; // the special module/class initialize function.
export const specialFunctionConstructor = ".ctor"; // the special class instance constructor function.

// A module contains other members, including submodules, variables, functions, and/or classes.
export interface Module extends Definitions {
    access?: Accessibility;

    description?: string; // an optional informative description.
}

// A variable is a typed storage location.
export interface Variable {
    type: symbols.TypeToken;

    default?: any;
    readonly?: boolean;

    description?: string; // an optional informative description.
}

// A simple variable is one that isn't a member of a class.
export interface SimpleVariable extends Variable {
    access?: Accessibility;
}

// A class property is just like a variable, but permits some extra attriubtes.
export interface ClassProperty extends Variable {
    access?: ClassMemberAccessibility;
    static?: boolean;
    primary?: boolean;
}

// A function is an executable bit of code: a class function, class method, or a lambda (see il module).
export interface Function {
    parameters?: Variable[];
    returnType?: symbols.TypeToken;

    body?: il.Block;

    description?: string; // an optional informative description.
}

// A simple function is one that isn't a member of a class.
export interface SimpleFunction extends Function {
    access?: Accessibility;
}

// A class method is just like a function, but permits some extra attributes.
export interface ClassMethod extends Function {
    access?: ClassMemberAccessibility;

    static?: boolean;
    sealed?: boolean;
    abstract?: boolean;
}

// A class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
export interface Class {
    access?: Accessibility;

    extends?: symbols.TypeToken;
    implements?: symbols.TypeToken[];

    sealed?: boolean;
    abstract?: boolean;
    record?: boolean;
    interface?: boolean;

    properties?: ClassProperty[];
    methods?: ClassMethod[];

    description?: string; // an optional informative description.
}

