// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as tokens from "../tokens";
import {Identifier, ModuleToken, Node, Token, TypeToken} from "./nodes";
import * as statements from "./statements";

// TODO(joe): consider refactoring modifiers from booleans to enums.

/* Definitions */

// A definition is something that possibly exported for external usage.
export interface Definition extends Node {
    name:         Identifier;  // a required name, unique amongst definitions with a common parent.
    description?: string;      // an optional informative description.
    attributes?:  Attribute[]; // an optional list of metadata attributes.
}

// An attribute is a simple decorator token that acts as a metadata annotation.
export interface Attribute extends Node {
    kind:      AttributeKind;
    decorator: Token;
}
export const attributeKind = "Attribute";
export type  AttributeKind = "Attribute";

/* Modules */

// A module contains members, including variables, functions, and/or classes.
export interface Module extends Definition {
    kind:     ModuleKind;
    exports?: ModuleExports; // a list of exported members, keyed by name.
    members?: ModuleMembers; // a list of members, keyed by their simple name.
}
export const moduleKind = "Module";
export type  ModuleKind = "Module";
export type  Modules = { [token: string /*tokens.ModuleToken*/]: Module };

// An export definition re-exports a definition from another module, possibly with a different name.
export interface Export extends Definition {
    kind:     ExportKind;
    referent: Token;
}
export const exportKind = "Export";
export type  ExportKind = "Export";

export type ModuleExports = { [token: string /*tokens.Name*/]: Export };

// A module member is a definition that belongs to a module.
export interface ModuleMember extends Definition {
}
export type ModuleMembers = { [token: string /*tokens.Name*/]: ModuleMember };

/* Classes */

// A class can be constructed to create an object, and exports properties, methods, and has a number of attributes.
export interface Class extends ModuleMember {
    kind:        ClassKind;
    extends?:    TypeToken;
    implements?: TypeToken[];
    sealed?:     boolean;
    abstract?:   boolean;
    record?:     boolean;
    interface?:  boolean;
    members?:    ClassMembers;
}
export const classKind = "Class";
export type  ClassKind = "Class";

// A class member is a definition that belongs to a class.
export interface ClassMember extends Definition {
    access?:  tokens.Accessibility;
    static?:  boolean;
    primary?: boolean;
}
export type ClassMembers = { [token: string /*tokens.Token*/]: ClassMember };

/* Variables */

// A variable is a typed storage location.
export interface Variable extends Definition {
    type:      TypeToken; // the required type token.
    default?:  any;       // a trivially serializable default value.
    readonly?: boolean;
}

// A variable that is lexically scoped within a function (either a parameter or local).
export interface LocalVariable extends Variable {
    kind: LocalVariableKind;
}
export const localVariableKind = "LocalVariable";
export type  LocalVariableKind = "LocalVariable";

// A module property is like a variable but belongs to a module.
export interface ModuleProperty extends Variable, ModuleMember {
    kind: ModulePropertyKind;
}
export const modulePropertyKind = "ModuleProperty";
export type  ModulePropertyKind = "ModuleProperty";

// A class property is just like a module property with some extra attributes.
export interface ClassProperty extends Variable, ClassMember {
    kind:      ClassPropertyKind;
    optional?: boolean;
}
export const classPropertyKind = "ClassProperty";
export type  ClassPropertyKind = "ClassProperty";

/* Functions */

// A function is an executable bit of code: a class function, class method, or a lambda (see il module).
export interface Function extends Definition {
    parameters?: LocalVariable[];
    returnType?: TypeToken;
    body?:       statements.Statement;
}

// A module method is just a function defined at the module scope.
export interface ModuleMethod extends Function, ModuleMember {
    kind: ModuleMethodKind;
}
export const moduleMethodKind = "ModuleMethod";
export type  ModuleMethodKind = "ModuleMethod";

// A class method is just like a module method with some extra attributes.
export interface ClassMethod extends Function, ClassMember {
    kind:      ClassMethodKind;
    sealed?:   boolean;
    abstract?: boolean;
}
export const classMethodKind = "ClassMethod";
export type  ClassMethodKind = "ClassMethod";

/** Helper functions **/

export function isDefinition(node: Node): boolean {
    switch (node.kind) {
        case moduleKind:
        case exportKind:
        case classKind:
        case localVariableKind:
        case modulePropertyKind:
        case classPropertyKind:
        case moduleMethodKind:
        case classMethodKind:
            return true;
        default:
            return false;
    }
}

