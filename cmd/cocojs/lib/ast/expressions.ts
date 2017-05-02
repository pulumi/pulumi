// Copyright 2017 Pulumi, Inc. All rights reserved.

import {LocalVariable} from "./definitions";
import {Identifier, Node, Token, TypeToken} from "./nodes";
import * as statements from "./statements";

import * as tokens from "../tokens";

export interface Expression extends Node {}

/** Literals **/

export interface Literal extends Expression {
    raw?: string; // the raw literal, for round tripping purposes.
}

// A `null` literal.
export interface NullLiteral extends Literal {
    kind: NullLiteralKind;
}
export const nullLiteralKind = "NullLiteral";
export type  NullLiteralKind = "NullLiteral";

// A `bool`-typed literal (`true` or `false`).
export interface BoolLiteral extends Literal {
    kind:  BoolLiteralKind;
    value: boolean;
}
export const boolLiteralKind = "BoolLiteral";
export type  BoolLiteralKind = "BoolLiteral";

// A `number`-typed literal (floating point IEEE 754).
export interface NumberLiteral extends Literal {
    kind:  NumberLiteralKind;
    value: number;
}
export const numberLiteralKind = "NumberLiteral";
export type  NumberLiteralKind = "NumberLiteral";

// A `string`-typed literal.
export interface StringLiteral extends Literal {
    kind:  StringLiteralKind;
    value: string;
}
export const stringLiteralKind = "StringLiteral";
export type  StringLiteralKind = "StringLiteral";

// An array literal plus optional initialization.
export interface ArrayLiteral extends Literal {
    kind:      ArrayLiteralKind;
    elemType?: TypeToken;    // the type of the array's elements.
    size?:     Expression;   // an optional expression for the array size.
    elements?: Expression[]; // an optional array of element expressions to store into the array.
}
export const arrayLiteralKind = "ArrayLiteral";
export type  ArrayLiteralKind = "ArrayLiteral";

// An object literal plus optional initialization.
export interface ObjectLiteral extends Literal {
    kind:        ObjectLiteralKind;
    type?:       TypeToken;               // the type of object to produce.
    properties?: ObjectLiteralProperty[]; // an optional array of property initializers.
}
export const objectLiteralKind = "ObjectLiteral";
export type  ObjectLiteralKind = "ObjectLiteral";

// An object literal property initializer.
export interface ObjectLiteralProperty extends Node {
    value:    Expression; // the expression value to store into the property.
}

// An named object literal property initializer.
export interface ObjectLiteralNamedProperty extends ObjectLiteralProperty {
    kind:     ObjectLiteralNamedPropertyKind;
    property: Token; // the name of the property being initialized.
}
export const objectLiteralNamedPropertyKind = "ObjectLiteralNamedProperty";
export type  ObjectLiteralNamedPropertyKind = "ObjectLiteralNamedProperty";

// A computed object literal property initializer.
export interface ObjectLiteralComputedProperty extends ObjectLiteralProperty {
    kind:     ObjectLiteralComputedPropertyKind;
    property: Expression; // the expression that evaluates to the property name being initialized.
}
export const objectLiteralComputedPropertyKind = "ObjectLiteralComputedProperty";
export type  ObjectLiteralComputedPropertyKind = "ObjectLiteralComputedProperty";

/** Loads **/

// TODO(joe): figure out how to load/store elements and maps.  Possibly just use intrinsic functions.

export interface LoadExpression extends Expression {
}

// Loads a location's address, producing a pointer that can be dereferenced.
export interface LoadLocationExpression extends LoadExpression {
    kind:    LoadLocationExpressionKind;
    object?: Expression; // the `this` object, in the case of class properties.
    name:    Token;      // the full token of the member to load.
}
export const loadLocationExpressionKind = "LoadLocationExpression";
export type  LoadLocationExpressionKind = "LoadLocationExpression";

// Dynamically loads either a variable or function, by name, from an object or scope, throwing an error if missing.
export interface LoadDynamicExpression extends LoadExpression {
    kind:    LoadDynamicExpressionKind;
    object?: Expression; // the object to load a property from.
    name:    Expression; // the name of the property to load.
}
export const loadDynamicExpressionKind = "LoadDynamicExpression";
export type  LoadDynamicExpressionKind = "LoadDynamicExpression";

// Dynamically loads either a variable or function, by name, from an object or scope, returning null if missing.
export interface TryLoadDynamicExpression extends LoadExpression {
    kind:    TryLoadDynamicExpressionKind;
    object?: Expression; // the object to load a property from.
    name:    Expression; // the name of the property to load.
}
export const tryLoadDynamicExpressionKind = "TryLoadDynamicExpression";
export type  TryLoadDynamicExpressionKind = "TryLoadDynamicExpression";

/** Functions **/

export interface CallExpression extends Expression {
    arguments?: CallArgument[]; // the list of arguments in sequential order.
}

export interface CallArgument extends Node {
    name?: Identifier; // a name if using named arguments.
    expr: Expression;  // the argument expression.
}
export const callArgumentKind = "CallArgument";
export type  CallArgumentKind = "CallArgument";

// Allocates a new object and calls its constructor.
export interface NewExpression extends CallExpression {
    kind: NewExpressionKind;
    type: TypeToken; // the object type to allocate.
}
export const newExpressionKind = "NewExpression";
export type  NewExpressionKind = "NewExpression";

// Invokes a function.
export interface InvokeFunctionExpression extends CallExpression {
    kind:     InvokeFunctionExpressionKind;
    function: Expression; // a function to invoke (of a func type).
}
export const invokeFunctionExpressionKind = "InvokeFunctionExpression";
export type  InvokeFunctionExpressionKind = "InvokeFunctionExpression";

// Creates a lambda, a sort of "anonymous" function.
export interface LambdaExpression extends Expression {
    kind:        LambdaExpressionKind;
    parameters?: LocalVariable[];   // the parameters.
    returnType?: tokens.TypeToken; // the optional return type.
    body:        statements.Block;  // the lambda's body block.
}
export const lambdaExpressionKind = "LambdaExpression";
export type  LambdaExpressionKind = "LambdaExpression";

/** Operators **/

// A unary operator expression.
export interface UnaryOperatorExpression extends Expression {
    kind:     UnaryOperatorExpressionKind;
    operator: UnaryOperator; // the operator type.
    operand:  Expression;    // the right hand side operand.
    postfix?: boolean;       // whether this is a postifx operator (only legal for UnaryPfixOperator).
}
export const unaryOperatorExpressionKind = "UnaryOperatorExpression";
export type  UnaryOperatorExpressionKind = "UnaryOperatorExpression";

// A unary prefix/postfix operator token.
export type UnaryPfixOperator = "++" | "--";

// A unary operator token.  Note that CocoIL doesn't care about precedence.  The CocoLang compilers must present the
// expressions in the order in which they should be evaluated through an in-order AST tree walk.
export type UnaryOperator = UnaryPfixOperator |
                            "*" | "&"         | // dereference and addressof.
                            "+" | "-"         | // unary plus and minus
                            "!" | "~"         ; // logical NOT and bitwise NOT.

// A binary operator expression (assignment, logical, operator, or relational).
export interface BinaryOperatorExpression extends Expression {
    kind:     BinaryOperatorExpressionKind;
    left:     Expression;     // the left hand side.
    operator: BinaryOperator; // the operator.
    right:    Expression;     // the right hand side.
}
export const binaryOperatorExpressionKind = "BinaryOperatorExpression";
export type  BinaryOperatorExpressionKind = "BinaryOperatorExpression";

// All of the available arithmetic operators.
export type BinaryArithmeticOperator  = "+"   | "-"   | // addition and subtraction.
                                        "*"   | "/"   | // multiplication and division.
                                        "%"   | "**"  ; // remainder and exponentiation.

// All of the available assignment operators.
// TODO: figure out what to do with ECMAScript's >>>= operator.
export type BinaryAssignmentOperator  = "="           | // simple assignment.
                                        "+="  | "-="  | // assignment by sum and difference.
                                        "*="  | "/="  | // assignment by product and quotient.
                                        "%="  | "**=" | // assignment by remainder and exponentiation.
                                        "<<=" | ">>=" | // assignment by bitwise left and right shift.
                                        "&="  | "|="  | // assignment by bitwise AND and OR.
                                        "^="          ; // assignment by bitwise XOR.

// All of the available bitwise operators.
// TODO: figure out what to do with ECMAScript's >>> operator.
export type BinaryBitwiseOperator     = "<<"  | ">>"  | // bitwise left and right shift.
                                        "&"   | "|"   | // bitwise AND and OR (inclusive OR).
                                        "^"           ; // bitwise XOR (exclusive OR).

// All of the available conditional operators.
export type BinaryConditionalOperator = "&&"  | "||"  ; // logical AND and OR.

// All of the available relational operators.
// TODO: figure out what to do with ECMAScript's === and !=== operators.
export type BinaryRelationalOperator  = "<"   | "<="  | // relational operators less-than and less-than-or-equals.
                                        ">"   | ">="  | // relational operators greater-than and greater-than-or-equals.
                                        "=="  | "!="  ; // relational operators equals and not-equals.

// A binary operator token.  Note that CocoIL doesn't care about precedence.  The CocoLang compilers must present the
// expressions in the order in which they should be evaluated through an in-order AST tree walk.
export type BinaryOperator = BinaryArithmeticOperator  |
                             BinaryAssignmentOperator  |
                             BinaryBitwiseOperator     |
                             BinaryConditionalOperator |
                             BinaryRelationalOperator  ;

/** Type Testing **/

// A cast expression; this handles both nominal and structural casts, and will throw an exception upon failure.
export interface CastExpression extends Expression {
    kind:       CastExpressionKind;
    expression: Expression; // the source expression.
    type:       TypeToken;  // the target type.
}
export const castExpressionKind = "CastExpression";
export type  CastExpressionKind = "CastExpression";

// An isinst expression checks an expression for compatibility with the given type token, evaluating to a boolean.
export interface IsInstExpression extends Expression {
    kind:       IsInstExpressionKind;
    expression: Expression; // the source expression.
    type:       TypeToken;  // the target type.
}
export const isInstExpressionKind = "IsInstExpression";
export type  IsInstExpressionKind = "IsInstExpression";

// A typeof instruction gets the type token -- just a string -- of a particular expression at runtime.
export interface TypeOfExpression extends Expression {
    kind:       TypeOfExpressionKind;
    expression: Expression;        // the source expression.
}
export const typeOfExpressionKind = "TypeOfExpression";
export type  TypeOfExpressionKind = "TypeOfExpression";

/** Miscellaneous **/

// A conditional expression.
export interface ConditionalExpression extends Expression {
    kind:       ConditionalExpressionKind;
    condition:  Expression; // a `bool` condition expression.
    consequent: Expression; // the expression to evaluate if `true`.
    alternate:  Expression; // the expression to evaluate if `false`.
}
export const conditionalExpressionKind = "ConditionalExpression";
export type  ConditionalExpressionKind = "ConditionalExpression";

// A sequence expression allows composition of multiple expressions into one.  It evaluates to the last one.
export interface SequenceExpression extends Expression {
    kind:        SequenceExpressionKind;
    expressions: Expression[];
}
export const sequenceExpressionKind = "SequenceExpression";
export type  SequenceExpressionKind = "SequenceExpression";

