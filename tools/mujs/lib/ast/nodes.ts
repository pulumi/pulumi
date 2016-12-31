// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as definitions from "./definitions";
import * as expressions from "./expressions";
import * as source from "./source";
import * as statements from "./statements";

// Node is a discriminated type for all serialized blocks and instructions.
export interface Node {
    kind: NodeKind;
    loc?: source.Location;
}

// NodeType contains all of the legal Node implementations.  This effectively "seales" the discriminated node type,
// and makes constructing and inspecting nodes a little more bulletproof (i.e., they aren't arbitrary strings).
export type NodeKind =
    definitions.ModuleKind |
    definitions.ParameterKind |
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
    statements.ExpressionStatementKind |

    expressions.NullLiteralExpressionKind |
    expressions.BoolLiteralExpressionKind |
    expressions.NumberLiteralExpressionKind |
    expressions.StringLiteralExpressionKind |
    expressions.ObjectLiteralExpressionKind |
    expressions.ObjectLiteralInitializerKind |
    expressions.LoadVariableExpressionKind |
    expressions.LoadFunctionExpressionKind |
    expressions.LoadDynamicExpressionKind |
    expressions.InvokeFunctionExpressionKind |
    expressions.LambdaExpressionKind |
    expressions.UnaryOperatorExpressionKind |
    expressions.BinaryOperatorExpressionKind |
    expressions.CastExpressionKind |
    expressions.ConditionalExpressionKind
;

