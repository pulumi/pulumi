// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as "il" from "../il";

// A top-level package definition.
export interface IPackage {
    name: string;         // a required fully qualified name.
    description?: string; // an optional informational description.
    author?: string;      // an optional author.
    website?: string;     // an optional website for additional information.
    license?: string;     // an optional license governing this package's usage.

    dependencies?: DependencyToken[];

    definitions?: IDefinitions;
}

export type Token = string;      // a valid symbol token.
export type ModuleToken = Token; // a symbol token that resolves to a module.
export type TypeToken = Token;   // a symbol token that resolves to a type.

// A set of module definitions; every package has an implicit top-level one, as does each submodule.
export interface IDefinitions {
    modules?:   Modules;   // submodules.
    variables?: Variables; // module-scoped variables.
    functions?: Functions; // module-scoped functions.
    classes?:   Classes;   // classes.
}

export type Identifier = string; // a valid identifier:  (letter|"_") (letter | digit | "_")*

export type Modules =   { [key: Identifier]: IModule };   // a map of module definitions, keyed by unique identifier.
export type Variables = { [key: Identifier]: IVariable }; // a map of variable definitions, keyed by unique identifier.
export type Functions = { [key: Identifier]: IFunction }; // a map of function definitions, keyed by unique identifier.
export type Classes =   { [key: Identifier]: IClass };    // a map of class definitions, keyed by unique identifier.

export type Accessibility = "public" | "private";                          // accessibility modifiers common to all.
export type ClassMemberAccessibility = "public" | "private" | "protected"; // accessibility modifiers for class members.

export const SpecialFunctionEntryPoint =  ".main"; // the special package entrypoint function.
export const SpecialFunctionInitializer = ".init"; // the special module/class initialize function.
export const SpecialFunctionConstructor = ".ctor"; // the special class instance constructor function.

// A module contains other members, including submodules, variables, functions, and/or classes.
export interface IModule extends IDefinitions {
    access?: Accessibility;

    description?: string; // an optional informative description.
}

// A variable is a typed storage location.
export interface IVariable {
    type: TypeToken;
    access?: Accessibility;

    default?: any;
    readonly?: boolean;

    description?: string; // an optional informative description.
}

// A class property is just like a variable, but permits some extra attriubtes.
export interface IClassProperty extends IVariable {
    access?: ClassMemberAccessibility;
    static?: boolean;
    primary?: boolean;
}

// A function is an executable bit of code: a class function, class method, or a lambda (see il module).
export interface IFunction {
    access?: Accessibility;

    parameters?: IVariable[];
    returnType?: TypeToken;

    body?: il.Body;

    description?: string; // an optional informative description.
}

// A class method is just like a function, but permits some extra attributes.
export interface IClassMethod extends IFunction {
    access?: ClassMemberAccessibility;

    static?: boolean;
    sealed?: boolean;
    abstract?: boolean;
}

// A class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
export interface IClass {
    access?: Accessibility;

    extends?: TypeToken;
    implements?: TypeToken[];

    sealed?: boolean;
    abstract?: boolean;
    record?: boolean;
    interface?: boolean;

    properties?: IClassProperty[];
    methods?: IClassMethod[];

    description?: string; // an optional informative description.
}

