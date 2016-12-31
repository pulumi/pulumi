// Copyright 2016 Marapongo, Inc. All rights reserved.

import {Node} from "./nodes";
import * as symbols from "../symbols";

// TODO(joe): consider refactoring modifiers from booleans to enums.

export interface Definition extends Node {
    description?: string; // an optional informative description.
}

export type Definitions = Map<symbols.Identifier, Definition>;

// A module contains other members, including submodules, variables, functions, and/or classes.
export interface Module extends Definition {
    kind:    ModuleKind;
    access?: Accessibility;
}
export const moduleKind = "Module";
export type  ModuleKind = "Module";

// A variable is a typed storage location.
export interface Variable extends Definition {
    type:      symbols.TypeToken;
    default?:  any;
    readonly?: boolean;
}

// A parameter is a variable used for functions.
export interface Parameter extends Variable {
    kind: ParameterKind;
    name: symbols.Identifier;
}
export const parameterKind = "Parameter";
export type  ParameterKind = "Parameter";

// A module property is like a variable but has an accessibility modifier.
export interface ModuleProperty extends Variable {
    kind:    SimpleVariableKind;
    access?: Accessibility;
}
export const simpleVariableKind = "SimpleVariable";
export type  SimpleVariableKind = "SimpleVariable";

// A class property is just like a module property with some extra attributes.
export interface ClassProperty extends Variable {
    access?:  ClassMemberAccessibility;
    static?:  boolean;
    primary?: boolean;
}
export const classPropertyKind = "ClassProperty";
export type  ClassPropertyKind = "ClassProperty";

// A function is an executable bit of code: a class function, class method, or a lambda (see il module).
export interface Function extends Definition {
    parameters?: Parameter[];
    returnType?: symbols.TypeToken;
    body?:       ast.Block;
}

// A module method is just a function with an accessibility.
export interface ModuleMethod extends Function {
    kind:    ModuleMethodKind;
    access?: Accessibility;
}
export const moduleMethodKind = "ModuleMethod";
export type  ModuleMethodKind = "ModuleMethod";

// A class method is just like a module method with some extra attributes.
export interface ClassMethod extends Function {
    kind:      ClassMethodKind;
    access?:   ClassMemberAccessibility;
    static?:   boolean;
    sealed?:   boolean;
    abstract?: boolean;
}
export const classMethodKind = "ClassMethod";
export type  ClassMethodKind = "ClassMethod";

// A class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
export interface Class extends Definition {
    kind:         ClassKind;
    access?:      Accessibility;
    extends?:     symbols.TypeToken;
    implements?:  symbols.TypeToken[];
    sealed?:      boolean;
    abstract?:    boolean;
    record?:      boolean;
    interface?:   boolean;
    properties?:  ClassProperty[];
    methods?:     ClassMethod[];
}
export const classKind = "Class";
export type  ClassKind = "Class";

