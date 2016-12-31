// Copyright 2016 Marapongo, Inc. All rights reserved.

import {Location} from "./source";
import * as symbols from "../symbols";

// Node is a discriminated type for all serialized blocks and instructions.
export interface Node {
    kind: NodeKind;
    loc?: Location;
}

// NodeType contains all of the legal Node implementations.  This effectively "seales" the discriminated node type,
// and makes constructing and inspecting nodes a little more bulletproof (i.e., they aren't arbitrary strings).
export type NodeKind =
    // # Statements

    // ## Blocks
    BlockKind |
    LocalVariableDeclarationKind |
    TryCatchFinallyKind |
    TryCatchBlockKind |

    // ## Branches
    BreakStatementKind |
    ContinueStatementKind |
    IfStatementKind |
    LabeledStatementKind |
    WhileStatementKind |

    // ## Miscellaneous
    ExpressionStatementKind |

    // # Expressions

    // ## Literals
    NullLiteralExpressionKind |
    BoolLiteralExpressionKind |
    NumberLiteralExpressionKind |
    StringLiteralExpressionKind |
    ObjectLiteralExpressionKind |
    ObjectLiteralInitializerKind |

    // ## Loads
    LoadVariableExpressionKind |
    LoadFunctionExpressionKind |
    LoadDynamicExpressionKind |

    // ## Functions
    InvokeFunctionExpressionKind |
    LambdaExpressionKind |

    // ## Operators
    UnaryOperatorExpressionKind |
    BinaryOperatorExpressionKind |

    // ## Miscellaneous
    CastExpressionKind |
    ConditionalExpressionKind
;

