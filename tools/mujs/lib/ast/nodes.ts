// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as definitions from "./definitions";
import * as expressions from "./expressions";
import * as source from "./source";
import * as statements from "./statements";

import * as symbols from "../symbols";

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

    definitions.ModuleKind |
    definitions.LocalVariableKind |
    definitions.ModulePropertyKind |
    definitions.ClassPropertyKind |
    definitions.ModuleMethodKind |
    definitions.ClassMethodKind |
    definitions.ClassKind |

    statements.BlockKind |
    statements.LocalVariableDeclarationKind |
    statements.TryCatchFinallyKind |
    statements.TryCatchBlockKind |
    statements.BreakStatementKind |
    statements.ContinueStatementKind |
    statements.IfStatementKind |
    statements.LabeledStatementKind |
    statements.ReturnStatementKind |
    statements.ThrowStatementKind |
    statements.WhileStatementKind |
    statements.EmptyStatementKind |
    statements.MultiStatementKind |
    statements.ExpressionStatementKind |

    expressions.NullLiteralKind |
    expressions.BoolLiteralKind |
    expressions.NumberLiteralKind |
    expressions.StringLiteralKind |
    expressions.ObjectLiteralKind |
    expressions.ObjectLiteralInitializerKind |
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
    ident: symbols.Token; // a valid identifier:  (letter|"_") (letter | digit | "_")*
}
export const identifierKind = "Identifier";
export type  IdentifierKind = "Identifier";

