// Copyright 2016 Marapongo, Inc. All rights reserved.

import {Expression} from "./expressions";
import {Node} from "./nodes";

import * as symbols from "../symbols";

export interface Statement extends Node {}

/** Blocks **/

export interface Block extends Statement {
    kind:       BlockKind;
    locals:     LocalVariableDeclaration[];
    statements: Statement[];
}
export const blockKind = "Block";
export type  BlockKind = "Block";

export interface LocalVariableDeclaration extends Node {
    kind: LocalVariableDeclarationKind;
    key:  symbols.VariableToken; // the token used to reference this local variable.
    type: symbols.TypeToken;     // the static type of this local variable's slot.
}
export const localVariableDeclarationKind = "LocalVariableDeclaration";
export type  LocalVariableDeclarationKind = "LocalVariableDeclaration";

/** Try/Catch/Finally **/

export interface TryCatchFinally extends Statement {
    kind:          TryCatchFinallyKind;
    tryBlock:      Block;
    catchBlocks?:  TryCatchBlock[];
    finallyBlock?: Block;
}
export const tryCatchFinallyKind = "TryCatchFinally";
export type  TryCatchFinallyKind = "TryCatchFinally";

export interface TryCatchBlock extends Node {
    kind:      TryCatchBlockKind;
    exception: symbols.TypeToken;
    block:     Block;
}
export const tryCatchBlockKind = "TryCatchBlock";
export type  TryCatchBlockKind = "TryCatchBlock";

/** Branches **/

// A `break` statement (valid only within loops).
export interface BreakStatement extends Statement {
    kind:   BreakStatementKind;
    label?: symbols.Identifier;
}
export const breakStatementKind = "BreakStatement";
export type  BreakStatementKind = "BreakStatement";

// A `continue` statement (valid only within loops).
export interface ContinueStatement extends Statement {
    kind:   ContinueStatementKind;
    label?: symbols.Identifier;
}
export const continueStatementKind = "ContinueStatement";
export type  ContinueStatementKind = "ContinueStatement";

// An `if` statement.  To simplify the AST, this is the only conditional statement available.  All higher-level
// conditional constructs such as `switch`, `if / else if / ...`, etc. must be desugared into it.
export interface IfStatement extends Statement {
    kind:       IfStatementKind;
    condition:  Expression; // a `bool` condition expression.
    consequent: Statement; // the statement to execute if `true`.
    alternate?: Statement; // the statement to execute if `false`.
}
export const ifStatementKind = "IfStatement";
export type  IfStatementKind = "IfStatement";

// A labeled statement associates an identifier with a statement for purposes of labeled jumps.
export interface LabeledStatement extends Statement {
    kind:      LabeledStatementKind;
    label:     symbols.Identifier;
    statement: Statement;
}
export const labeledStatementKind = "LabeledStatement";
export type  LabeledStatementKind = "LabeledStatement";

// A `while` statement.  To simplify the AST, this is the only looping statement available.  All higher-level
// looping constructs such as `for`, `foreach`, `for in`, `for of`, `do / while`, etc. must be desugared into it.
export interface WhileStatement extends Statement {
    kind: WhileStatementKind;
    test: Expression; // a `bool` statement indicating whether to continue.
}
export const whileStatementKind = "WhileStatement";
export type  WhileStatementKind = "WhileStatement";

/** Miscellaneous **/

// A statement that performs an expression, but ignores its result.
export interface ExpressionStatement extends Statement {
    kind:       ExpressionStatementKind;
    expression: Expression;
}
export const expressionStatementKind = "ExpressionStatement";
export type  ExpressionStatementKind = "ExpressionStatement";

