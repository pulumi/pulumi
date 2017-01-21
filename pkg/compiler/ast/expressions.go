// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"github.com/marapongo/mu/pkg/tokens"
)

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
	Type     *tokens.Type  `json:"type,omitempty"`     // the optional type of array being produced.
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
	Type       *tokens.Type              `json:"type,omitempty"`       // the optional type of object to produce.
	Properties *[]*ObjectLiteralProperty `json:"properties,omitempty"` // an optional array of property initializers.
}

var _ Node = (*ObjectLiteral)(nil)
var _ Expression = (*ObjectLiteral)(nil)
var _ Literal = (*ObjectLiteral)(nil)

const ObjectLiteralKind NodeKind = "ObjectLiteral"

// ObjectLiteralProperty initializes a single object literal property.
type ObjectLiteralProperty struct {
	NodeValue
	Name  *Identifier `json:"name"`  // the property to initialize.
	Value Expression  `json:"value"` // the expression whose value to store into the property.
}

var _ Node = (*ObjectLiteralProperty)(nil)

const ObjectLiteralPropertyKind NodeKind = "ObjectLiteralProperty"

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
	Name   *Identifier `json:"name"`             // the name of the member to load.
}

var _ Node = (*LoadLocationExpression)(nil)
var _ Expression = (*LoadLocationExpression)(nil)
var _ LoadExpression = (*LoadLocationExpression)(nil)

const LoadLocationExpressionKind NodeKind = "LoadLocationExpression"

// LoadDynamicExpression dynamically loads either a variable or a function, by name, from an object.
type LoadDynamicExpression struct {
	loadExpressionNode
	Object Expression `json:"object"` // the object from which to load the property.
	Name   Expression `json:"name"`   // the dynamically evaluated name of the property to load.
}

var _ Node = (*LoadDynamicExpression)(nil)
var _ Expression = (*LoadDynamicExpression)(nil)
var _ LoadExpression = (*LoadDynamicExpression)(nil)

const LoadDynamicExpressionKind NodeKind = "LoadDynamicExpression"

/* Functions */

type CallExpression interface {
	Expression
	GetArguments() *[]Expression // the list of arguments in sequential order.
}

type callExpressionNode struct {
	ExpressionNode
	Arguments *[]Expression `json:"arguments,omitempty"`
}

func (node *callExpressionNode) GetArguments() *[]Expression { return node.Arguments }

// NewExpression allocates a new object and calls its constructor.
type NewExpression struct {
	callExpressionNode
	Type *Identifier `json:"type"` // the object type to allocate.
}

var _ Node = (*NewExpression)(nil)
var _ Expression = (*NewExpression)(nil)
var _ CallExpression = (*NewExpression)(nil)

const NewExpressionKind NodeKind = "NewExpression"

// InvokeFunction invokes a target expression that must evaluate to a function.
type InvokeFunctionExpression struct {
	callExpressionNode
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
var _ Expression = (*LambdaExpression)(nil)

const LambdaExpressionKind NodeKind = "LambdaExpression"

/* Operators */

// UnaryOperatorExpression is the usual C-like unary operator.
type UnaryOperatorExpression struct {
	ExpressionNode
	Operator UnaryOperator `json:"operator"` // the operator type.
	Operand  Expression    `json:"operand"`  // the right hand side operand.
	Postfix  bool          `json:"postfix"`  // whether this is a postfix operator (only legal for UnaryPfixOperators).
}

var _ Node = (*UnaryOperatorExpression)(nil)
var _ Expression = (*UnaryOperatorExpression)(nil)

const UnaryOperatorExpressionKind NodeKind = "UnaryOperatorExpression"

// UnaryOperator is the full set of unary operator tokens.  Note that MuIL doesn't care about precedence.  The MetaMu
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

// All of the available arithmetic operators.
type BinaryOperator string

const (
	// Arithmetic operators:
	OpAdd          BinaryOperator = "+"
	OpSubtract                    = "-"
	OpMultiply                    = "*"
	OpDivide                      = "/"
	OpRemainder                   = "%"
	OpExponentiate                = "**"

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

	// Bitwise operators:
	OpBitwiseShiftLeft  = "<<"
	OpBitwiseShiftRight = ">>"
	OpBitwiseAnd        = "&"
	OpBitwiseOr         = "|"
	OpBitwiseXor        = "^"

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
	Expression Expression  `json:"expression"` // the source expression.
	Type       tokens.Type `json:"type"`       // the target type.
}

var _ Node = (*CastExpression)(nil)
var _ Expression = (*CastExpression)(nil)

const CastExpressionKind NodeKind = "CastExpression"

// IsInstExpression checks an expression for compatibility with the given type token, and evaluates to a bool.
type IsInstExpression struct {
	ExpressionNode
	Expression Expression  `json:"expression"` // the source expression.
	Type       tokens.Type `json:"type"`       // the target type.
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

// SequenceExpression allows composition of multiple expressions into one.  It evaluates to the last one.
type SequenceExpression struct {
	ExpressionNode
	Expressions []Expression `json:"expressions"`
}

var _ Node = (*SequenceExpression)(nil)
var _ Expression = (*SequenceExpression)(nil)

const SequenceExpressionKind NodeKind = "SequenceExpression"
