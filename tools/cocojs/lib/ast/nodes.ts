// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as definitions from "./definitions";
import * as expressions from "./expressions";
import * as source from "./source";
import * as statements from "./statements";

import * as tokens from "../tokens";

// TODO(joe): consider adding trivia (like comments and whitespace), for round-tripping purposes.

// Node is a discriminated type for all serialized blocks and instructions.
export interface Node {
    kind: NodeKind;
    loc?: source.Location;
}

// NodeType contains all of the legal Node implementations.  This effectively "seales" the discriminated node type,
// and makes constructing and inspecting nodes a little more bulletproof (i.e., they aren't arbitrary strings).
export type NodeKind =
    IdentifierKind |
    TokenKind |
    ClassMemberTokenKind |
    ModuleTokenKind |
    TypeTokenKind |

    definitions.ModuleKind |
    definitions.ClassKind |
    definitions.ExportKind |
    definitions.LocalVariableKind |
    definitions.ClassPropertyKind |
    definitions.ModulePropertyKind |
    definitions.ClassMethodKind |
    definitions.ModuleMethodKind |

    statements.BlockKind |
    statements.LocalVariableDeclarationKind |
    statements.TryCatchFinallyKind |
    statements.TryCatchBlockKind |
    statements.BreakStatementKind |
    statements.ContinueStatementKind |
    statements.IfStatementKind |
    statements.SwitchStatementKind |
    statements.SwitchCaseKind |
    statements.LabeledStatementKind |
    statements.ReturnStatementKind |
    statements.ThrowStatementKind |
    statements.WhileStatementKind |
    statements.ForStatementKind |
    statements.EmptyStatementKind |
    statements.MultiStatementKind |
    statements.ExpressionStatementKind |

    expressions.NullLiteralKind |
    expressions.BoolLiteralKind |
    expressions.NumberLiteralKind |
    expressions.StringLiteralKind |
    expressions.ArrayLiteralKind |
    expressions.ObjectLiteralKind |
    expressions.ObjectLiteralPropertyKind |
    expressions.LoadLocationExpressionKind |
    expressions.LoadDynamicExpressionKind |
    expressions.NewExpressionKind |
    expressions.InvokeFunctionExpressionKind |
    expressions.LambdaExpressionKind |
    expressions.UnaryOperatorExpressionKind |
    expressions.BinaryOperatorExpressionKind |
    expressions.CastExpressionKind |
    expressions.IsInstExpressionKind |
    expressions.TypeOfExpressionKind |
    expressions.ConditionalExpressionKind |
    expressions.SequenceExpressionKind
;

export interface Identifier extends Node {
    kind:  IdentifierKind;
    ident: string; // a valid identifier:  (letter|"_") (letter | digit | "_")*
}
export const identifierKind = "Identifier";
export type  IdentifierKind = "Identifier";

export interface Token extends Node {
    kind: TokenKind;
    tok:  tokens.Token;
}
export const tokenKind = "Token";
export type  TokenKind = "Token";

export interface ClassMemberToken extends Node {
    kind: ClassMemberTokenKind;
    tok:  tokens.ClassMemberToken;
}
export const classMemberTokenKind = "ClassMemberToken";
export type  ClassMemberTokenKind = "ClassMemberToken";

export interface ModuleToken extends Node {
    kind: ModuleTokenKind;
    tok:  tokens.ModuleToken;
}
export const moduleTokenKind = "ModuleToken";
export type  ModuleTokenKind = "ModuleToken";

export interface TypeToken extends Node {
    kind: TypeTokenKind;
    tok:  tokens.TypeToken;
}
export const typeTokenKind = "TypeToken";
export type  TypeTokenKind = "TypeToken";

