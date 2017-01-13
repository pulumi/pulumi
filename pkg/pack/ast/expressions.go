// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"github.com/marapongo/mu/pkg/pack/symbols"
)

// Expression is an executable operation that usually produces a value.
type Expression interface {
	Node
	expression()
}

type expression struct {
	node
}

func (node *expression) expression() {}

/* Literals */

type Literal interface {
	GetRaw() *string // the raw literal, for round tripping purposes.
}

type literal struct {
	expression
	Raw *string `json:"raw,omitempty"`
}

func (node *literal) GetRaw() *string { return node.Raw }

// NullLiteral represents the usual `null` constant.
type NullLiteral struct {
	literal
}

// BoolLiteral represents the usual Boolean literal constant (`true` or `false`).
type BoolLiteral struct {
	literal
	Value bool `json:"value"`
}

// NumberLiteral represents a floating point IEEE 754 literal value.
type NumberLiteral struct {
	literal
	Value float64 `json:"value"`
}

// StringLiteral represents a UTF8-encoded string literal.
type StringLiteral struct {
	literal
	Value string `json:"value"`
}

// ArrayLiteral evaluates to a newly allocated array, with optional initialized elements.
type ArrayLiteral struct {
	literal
	Type     *symbols.TypeToken `json:"type,omitempty"`     // the optional type of array being produced.
	Size     *Expression        `json:"size,omitempty"`     // an optional expression for the array size.
	Elements *[]Expression      `json:"elements,omitempty"` // an optional array of element initializers.
}

// ObjectLiteral evaluates to a new object, with optional property initializers for primary properties.
type ObjectLiteral struct {
	literal
	Type       *symbols.TypeToken       `json:"type,omitempty"`       // the optional type of object to produce.
	Properties *[]ObjectLiteralProperty `json:"properties,omitempty"` // an optional array of property initializers.
}

// ObjectLiteralProperty initializes a single object literal property.
type ObjectLiteralProperty struct {
	node
	Name  *Identifier `json:"name"`  // the property to initialize.
	Value Expression  `json:"value"` // the expression whose value to store into the property.
}

/* Loads */

type LoadExpression interface {
	Expression
	loadExpression()
}

type loadExpression struct {
	expression
}

func (node *loadExpression) loadExpression() {}

// LoadLocationExpression loads a location's address, producing a pointer that can be dereferenced.
type LoadLocationExpression struct {
	loadExpression
	Object *Expression `json:"object,omitempty"` // the `this` object, in the case of class properties.
	Name   *Identifier `json:"name"`             // the name of the member to load.
}

// LoadDynamicExpression dynamically loads either a variable or a function, by name, from an object.
type LoadDynamicExpression struct {
	loadExpression
	Object Expression `json:"object"` // the object from which to load the property.
	Name   Expression `json:"name"`   // the dynamically evaluated name of the property to load.
}

/* Functions */

type CallExpression interface {
	Expression
	GetArguments() *[]Expression // the list of arguments in sequential order.
}

type callExpression struct {
	expression
	Arguments *[]Expression `json:"arguments,omitempty"`
}

// NewExpression allocates a new object and calls its constructor.
type NewExpression struct {
	callExpression
	Type *Identifier // the object type to allocate.
}

// InvokeFunction invokes a target expression that must evaluate to a function.
type InvokeFunctionExpression struct {
	callExpression
	Function Expression `json:"function"` // a function to invoke (of a function type).
}

// LambdaExpression creates a lambda, a sort of "anonymous" function, that evaluates to a function type.
type LambdaExpression struct {
	expression
	Signature  symbols.TypeToken       `json:"signature"`  // the function signature type.
	Parameters []symbols.VariableToken `json:"parameters"` // the parameter variables.
	Body       *Block                  `json:"body"`       // the lambda's body block.
}

/* Operators */

// UnaryOperatorExpression is the usual C-like unary operator.
type UnaryOperatorExpression struct {
	expression
	Operator UnaryOperator `json:"operator"` // the operator type.
	Operand  Expression    `json:"operand"`  // the right hand side operand.
	Postfix  bool          `json:"postfix"`  // whether this is a postfix operator (only legal for UnaryPfixOperators).
}

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
	expression
	Left     Expression     `json:"left"`     // the left hand side.
	Operator BinaryOperator `json:"operator"` // the operator.
	Right    Expression     `json:"right"`    // the right hand side.
}

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
	expression
	Expression Expression        `json:"expression"` // the source expression.
	Type       symbols.TypeToken `json:"type"`       // the target type.
}

// IsInstExpression checks an expression for compatibility with the given type token, and evaluates to a bool.
type IsInstExpression struct {
	expression
	Expression Expression        `json:"expression"` // the source expression.
	Type       symbols.TypeToken `json:"type"`       // the target type.
}

// TypeOfExpression gets the type token -- just a string -- of a particular type at runtime.
type TypeOfExpression struct {
	expression
	Expression Expression `json:"expression"` // the source expression
}

/* Miscellaneous */

// ConditionalExpression evaluates to either a consequent or alternate based on a predicate condition.
type ConditionalExpression struct {
	expression
	Condition  Expression `json:"condition"`  // a `bool` conditional expression.
	Consequent Expression `json:"consequent"` // the expression to evaluate to if `true`.
	Alternate  Expression `json:"alternate"`  // the expression to evaluate to if `false`.
}

// SequenceExpression allows composition of multiple expressions into one.  It evaluates to the last one.
type SequenceExpression struct {
	expression
	Expressions []Expression `json:"expressions"`
}
