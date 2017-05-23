// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ast

// Expression is an executable operation that usually produces a value.
type Expression interface {
	Node
	expression()
}

type ExpressionNode struct {
	NodeValue
}

func (node *ExpressionNode) expression() {}

/* Literals */

type Literal interface {
	GetRaw() *string // the raw literal, for round tripping purposes.
}

type LiteralNode struct {
	ExpressionNode
	Raw *string `json:"raw,omitempty"`
}

func (node *LiteralNode) GetRaw() *string { return node.Raw }

// NullLiteral represents the usual `null` constant.
type NullLiteral struct {
	LiteralNode
}

var _ Node = (*NullLiteral)(nil)
var _ Expression = (*NullLiteral)(nil)
var _ Literal = (*NullLiteral)(nil)

const NullLiteralKind NodeKind = "NullLiteral"

// BoolLiteral represents the usual Boolean literal constant (`true` or `false`).
type BoolLiteral struct {
	LiteralNode
	Value bool `json:"value"`
}

var _ Node = (*BoolLiteral)(nil)
var _ Expression = (*BoolLiteral)(nil)
var _ Literal = (*BoolLiteral)(nil)

const BoolLiteralKind NodeKind = "BoolLiteral"

// NumberLiteral represents a floating point IEEE 754 literal value.
type NumberLiteral struct {
	LiteralNode
	Value float64 `json:"value"`
}

var _ Node = (*NumberLiteral)(nil)
var _ Expression = (*NumberLiteral)(nil)
var _ Literal = (*NumberLiteral)(nil)

const NumberLiteralKind NodeKind = "NumberLiteral"

// StringLiteral represents a UTF8-encoded string literal.
type StringLiteral struct {
	LiteralNode
	Value string `json:"value"`
}

var _ Node = (*StringLiteral)(nil)
var _ Expression = (*StringLiteral)(nil)
var _ Literal = (*StringLiteral)(nil)

const StringLiteralKind NodeKind = "StringLiteral"

// ArrayLiteral evaluates to a newly allocated array, with optional initialized elements.
type ArrayLiteral struct {
	LiteralNode
	ElemType *TypeToken    `json:"elemType,omitempty"` // the optional type of array elements.
	Size     *Expression   `json:"size,omitempty"`     // an optional expression for the array size.
	Elements *[]Expression `json:"elements,omitempty"` // an optional array of element initializers.
}

var _ Node = (*ArrayLiteral)(nil)
var _ Expression = (*ArrayLiteral)(nil)
var _ Literal = (*ArrayLiteral)(nil)

const ArrayLiteralKind NodeKind = "ArrayLiteral"

// ObjectLiteral evaluates to a new object, with optional property initializers for primary properties.
type ObjectLiteral struct {
	LiteralNode
	Type       *TypeToken               `json:"type,omitempty"`       // the optional type of object to produce.
	Properties *[]ObjectLiteralProperty `json:"properties,omitempty"` // an optional array of property initializers.
}

var _ Node = (*ObjectLiteral)(nil)
var _ Expression = (*ObjectLiteral)(nil)
var _ Literal = (*ObjectLiteral)(nil)

const ObjectLiteralKind NodeKind = "ObjectLiteral"

// ObjectLiteralProperty initializes a single object literal property.
type ObjectLiteralProperty interface {
	Node
	Val() Expression
}

// ObjectLiteralNamedProperty initializes a single object literal property by name.
type ObjectLiteralNamedProperty struct {
	NodeValue
	Property *Token     `json:"property"` // the property (simple name if dynamic; member token otherwise).
	Value    Expression `json:"value"`    // the expression whose value to store into the property.
}

var _ Node = (*ObjectLiteralNamedProperty)(nil)
var _ ObjectLiteralProperty = (*ObjectLiteralNamedProperty)(nil)

const ObjectLiteralNamedPropertyKind NodeKind = "ObjectLiteralNamedProperty"

func (p *ObjectLiteralNamedProperty) Val() Expression { return p.Value }

// ObjectLiteralComputedProperty initializes a single object literal property, dynamically, through a computed name.
type ObjectLiteralComputedProperty struct {
	NodeValue
	Property Expression `json:"property"` // the property (simple name if dynamic; member token otherwise).
	Value    Expression `json:"value"`    // the expression whose value to store into the property.
}

var _ Node = (*ObjectLiteralComputedProperty)(nil)
var _ ObjectLiteralProperty = (*ObjectLiteralComputedProperty)(nil)

const ObjectLiteralComputedPropertyKind NodeKind = "ObjectLiteralComputedProperty"

func (p *ObjectLiteralComputedProperty) Val() Expression { return p.Value }

/* Loads */

type LoadExpression interface {
	Expression
	loadExpression()
}

type loadExpressionNode struct {
	ExpressionNode
}

func (node *loadExpressionNode) loadExpression() {}

// LoadLocationExpression loads a location's address, producing a pointer that can be dereferenced.
type LoadLocationExpression struct {
	loadExpressionNode
	Object *Expression `json:"object,omitempty"` // the `this` object, in the case of class properties.
	Name   *Token      `json:"name"`             // the token of the member or local variable to load.
}

var _ Node = (*LoadLocationExpression)(nil)
var _ Expression = (*LoadLocationExpression)(nil)
var _ LoadExpression = (*LoadLocationExpression)(nil)

const LoadLocationExpressionKind NodeKind = "LoadLocationExpression"

// LoadDynamicExpression dynamically loads either a variable or a function, by name, from an object or scope.
type LoadDynamicExpression struct {
	loadExpressionNode
	Object *Expression `json:"object,omitempty"` // the object from which to load the property.
	Name   Expression  `json:"name"`             // the dynamically evaluated name of the property to load.
}

var _ Node = (*LoadDynamicExpression)(nil)
var _ Expression = (*LoadDynamicExpression)(nil)
var _ LoadExpression = (*LoadDynamicExpression)(nil)

const LoadDynamicExpressionKind NodeKind = "LoadDynamicExpression"

// TryLoadDynamicExpression dynamically loads either a variable or a function, by name, from an object or scope; it is
// like LoadDynamicExpression, except that if the load fails, a null is produced instead of an exception.
type TryLoadDynamicExpression struct {
	loadExpressionNode
	Object *Expression `json:"object,omitempty"` // the object from which to load the property.
	Name   Expression  `json:"name"`             // the dynamically evaluated name of the property to load.
}

var _ Node = (*TryLoadDynamicExpression)(nil)
var _ Expression = (*TryLoadDynamicExpression)(nil)
var _ LoadExpression = (*TryLoadDynamicExpression)(nil)

const TryLoadDynamicExpressionKind NodeKind = "TryLoadDynamicExpression"

/* Functions */

type CallExpression interface {
	Expression
	GetArguments() *[]*CallArgument // the list of arguments in sequential order.
}

type CallArgument struct {
	NodeValue
	Name *Identifier `json:"name,omitempty"` // a name if using named arguments.
	Expr Expression  `json:"expr"`           // the argument expression.
}

var _ Node = (*CallArgument)(nil)

const CallArgumentKind NodeKind = "CallArgument"

type CallExpressionNode struct {
	ExpressionNode
	Arguments *[]*CallArgument `json:"arguments,omitempty"`
}

func (node *CallExpressionNode) GetArguments() *[]*CallArgument { return node.Arguments }

// NewExpression allocates a new object and calls its constructor.
type NewExpression struct {
	CallExpressionNode
	Type *TypeToken `json:"type"` // the object type to allocate.
}

var _ Node = (*NewExpression)(nil)
var _ Expression = (*NewExpression)(nil)
var _ CallExpression = (*NewExpression)(nil)

const NewExpressionKind NodeKind = "NewExpression"

// InvokeFunctionExpression invokes a target expression that must evaluate to a function.
type InvokeFunctionExpression struct {
	CallExpressionNode
	Function Expression `json:"function"` // a function to invoke (of a function type).
}

var _ Node = (*InvokeFunctionExpression)(nil)
var _ Expression = (*InvokeFunctionExpression)(nil)
var _ CallExpression = (*InvokeFunctionExpression)(nil)

const InvokeFunctionExpressionKind NodeKind = "InvokeFunctionExpression"

// LambdaExpression creates a lambda, a sort of "anonymous" function, that evaluates to a function type.
type LambdaExpression struct {
	ExpressionNode
	FunctionNode
}

var _ Node = (*LambdaExpression)(nil)
var _ Function = (*LambdaExpression)(nil)
var _ Expression = (*LambdaExpression)(nil)

const LambdaExpressionKind NodeKind = "LambdaExpression"

/* Operators */

// UnaryOperatorExpression is the usual C-like unary operator.
type UnaryOperatorExpression struct {
	ExpressionNode
	Operator UnaryOperator `json:"operator"`          // the operator type.
	Operand  Expression    `json:"operand"`           // the right hand side operand.
	Postfix  bool          `json:"postfix,omitempty"` // whether this is a postfix operator (just for pfix operators).
}

var _ Node = (*UnaryOperatorExpression)(nil)
var _ Expression = (*UnaryOperatorExpression)(nil)

const UnaryOperatorExpressionKind NodeKind = "UnaryOperatorExpression"

// UnaryOperator is the full set of unary operator tokens.  Note that LumiIL doesn't care about precedence.  LumiLang
// compilers must present expression in the order in which they should be evaluated through an in-order AST tree walk.
type UnaryOperator string

const (
	// Prefix-only operators:

	OpDereference UnaryOperator = "*"
	OpAddressof                 = "&"
	OpUnaryPlus                 = "+"
	OpUnaryMinus                = "-"
	OpLogicalNot                = "!"
	OpBitwiseNot                = "~"

	// These are permitted to be prefix or postfix:

	OpPlusPlus   = "++"
	OpMinusMinus = "--"
)

// BinaryOperatorExpression is the usual C-like binary operator (assignment, logical, operator, or relational).
type BinaryOperatorExpression struct {
	ExpressionNode
	Left     Expression     `json:"left"`     // the left hand side.
	Operator BinaryOperator `json:"operator"` // the operator.
	Right    Expression     `json:"right"`    // the right hand side.
}

var _ Node = (*BinaryOperatorExpression)(nil)
var _ Expression = (*BinaryOperatorExpression)(nil)

const BinaryOperatorExpressionKind NodeKind = "BinaryOperatorExpression"

// BinaryOperator is an enumeration of all of the available arithmetic operators.
type BinaryOperator string

const (
	// Arithmetic operators:

	OpAdd          BinaryOperator = "+"
	OpSubtract                    = "-"
	OpMultiply                    = "*"
	OpDivide                      = "/"
	OpRemainder                   = "%"
	OpExponentiate                = "**"

	// Bitwise operators:

	OpBitwiseShiftLeft  = "<<"
	OpBitwiseShiftRight = ">>"
	OpBitwiseAnd        = "&"
	OpBitwiseOr         = "|"
	OpBitwiseXor        = "^"

	// Assignment operators:

	OpAssign                  = "="
	OpAssignSum               = "+="
	OpAssignDifference        = "-="
	OpAssignProduct           = "*="
	OpAssignQuotient          = "/="
	OpAssignRemainder         = "%="
	OpAssignExponentiation    = "**="
	OpAssignBitwiseShiftLeft  = "<<="
	OpAssignBitwiseShiftRight = ">>="
	OpAssignBitwiseAnd        = "&="
	OpAssignBitwiseOr         = "|="
	OpAssignBitwiseXor        = "^="

	// Conditional operators:

	OpLogicalAnd = "&&"
	OpLogicalOr  = "||"

	// Relational operators:

	OpLt        = "<"
	OpLtEquals  = "<="
	OpGt        = ">"
	OpGtEquals  = ">="
	OpEquals    = "=="
	OpNotEquals = "!="
)

/* Type Testing */

// CastExpression handles both nominal and structural casts, and will throw an exception upon failure.
type CastExpression struct {
	ExpressionNode
	Expression Expression `json:"expression"` // the source expression.
	Type       *TypeToken `json:"type"`       // the target type.
}

var _ Node = (*CastExpression)(nil)
var _ Expression = (*CastExpression)(nil)

const CastExpressionKind NodeKind = "CastExpression"

// IsInstExpression checks an expression for compatibility with the given type token, and evaluates to a bool.
type IsInstExpression struct {
	ExpressionNode
	Expression Expression `json:"expression"` // the source expression.
	Type       *TypeToken `json:"type"`       // the target type.
}

var _ Node = (*IsInstExpression)(nil)
var _ Expression = (*IsInstExpression)(nil)

const IsInstExpressionKind NodeKind = "IsInstExpression"

// TypeOfExpression gets the type token -- just a string -- of a particular type at runtime.
type TypeOfExpression struct {
	ExpressionNode
	Expression Expression `json:"expression"` // the source expression
}

var _ Node = (*TypeOfExpression)(nil)
var _ Expression = (*TypeOfExpression)(nil)

const TypeOfExpressionKind NodeKind = "TypeOfExpression"

/* Miscellaneous */

// ConditionalExpression evaluates to either a consequent or alternate based on a predicate condition.
type ConditionalExpression struct {
	ExpressionNode
	Condition  Expression `json:"condition"`  // a `bool` conditional expression.
	Consequent Expression `json:"consequent"` // the expression to evaluate to if `true`.
	Alternate  Expression `json:"alternate"`  // the expression to evaluate to if `false`.
}

var _ Node = (*ConditionalExpression)(nil)
var _ Expression = (*ConditionalExpression)(nil)

const ConditionalExpressionKind NodeKind = "ConditionalExpression"

// SequenceExpression allows evaluation of multiple "prelude" statements and/or expressions as though they were a
// single expression.  The overall sequence evaluates to the final "value" expression.
type SequenceExpression struct {
	ExpressionNode
	Prelude []Node     `json:"prelude"`
	Value   Expression `json:"value"`
}

var _ Node = (*SequenceExpression)(nil)
var _ Expression = (*SequenceExpression)(nil)

const SequenceExpressionKind NodeKind = "SequenceExpression"
