package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// Expression represents a semantically-analyzed HCL2 expression.
type Expression = model.Expression

// AnonymousFunctionExpression represents a semantically-analyzed anonymous function expression.
// 
// These expressions are not the result of semantically analyzing syntax nodes. Instead, they may be synthesized by
// transforms over the IR for a program (e.g. the Apply transform).
type AnonymousFunctionExpression = model.AnonymousFunctionExpression

// BinaryOpExpression represents a semantically-analyzed binary operation.
type BinaryOpExpression = model.BinaryOpExpression

// ConditionalExpression represents a semantically-analzed conditional expression (i.e.
// <condition> '?' <true> ':' <false>).
type ConditionalExpression = model.ConditionalExpression

// ErrorExpression represents an expression that could not be bound due to an error.
type ErrorExpression = model.ErrorExpression

// ForExpression represents a semantically-analyzed for expression.
type ForExpression = model.ForExpression

// FunctionCallExpression represents a semantically-analyzed function call expression.
type FunctionCallExpression = model.FunctionCallExpression

// IndexExpression represents a semantically-analyzed index expression.
type IndexExpression = model.IndexExpression

// LiteralValueExpression represents a semantically-analyzed literal value expression.
type LiteralValueExpression = model.LiteralValueExpression

// ObjectConsItem records a key-value pair that is part of object construction expression.
type ObjectConsItem = model.ObjectConsItem

// ObjectConsExpression represents a semantically-analyzed object construction expression.
type ObjectConsExpression = model.ObjectConsExpression

// RelativeTraversalExpression represents a semantically-analyzed relative traversal expression.
type RelativeTraversalExpression = model.RelativeTraversalExpression

// ScopeTraversalExpression represents a semantically-analyzed scope traversal expression.
type ScopeTraversalExpression = model.ScopeTraversalExpression

type SplatVariable = model.SplatVariable

// SplatExpression represents a semantically-analyzed splat expression.
type SplatExpression = model.SplatExpression

// TemplateExpression represents a semantically-analyzed template expression.
type TemplateExpression = model.TemplateExpression

// TemplateJoinExpression represents a semantically-analyzed template join expression.
type TemplateJoinExpression = model.TemplateJoinExpression

// TupleConsExpression represents a semantically-analyzed tuple construction expression.
type TupleConsExpression = model.TupleConsExpression

// UnaryOpExpression represents a semantically-analyzed unary operation.
type UnaryOpExpression = model.UnaryOpExpression

func HCLExpression(x Expression) hcl.Expression {
	return model.HCLExpression(x)
}

