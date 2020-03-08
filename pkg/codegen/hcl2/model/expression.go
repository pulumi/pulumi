// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/resource"
)

// Expression represents a semantically-analyzed HCL2 expression.
type Expression interface {
	// SyntaxNode returns the hclsyntax.Node associated with the expression.
	SyntaxNode() hclsyntax.Node
	// Type returns the type of the expression.
	Type() Type

	isExpression()
}

// AnonymousFunctionExpression represents a semantically-analyzed anonymous function expression.
//
// These expressions are not the result of semantically analyzing syntax nodes. Instead, they may be synthesized by
// transforms over the IR for a program (e.g. the Apply transform).
type AnonymousFunctionExpression struct {
	// The signature for the anonymous function.
	Signature StaticFunctionSignature
	// The parameter definitions for the anonymous function.
	Parameters []*Variable

	// The body of the anonymous function.
	Body Expression
}

// SyntaxNode returns the syntax node associated with the body of the anonymous function.
func (x *AnonymousFunctionExpression) SyntaxNode() hclsyntax.Node {
	return x.Body.SyntaxNode()
}

// Type returns the type of the anonymous function expression.
//
// TODO: currently this returns the any type. Instead, it should return a function type.
func (x *AnonymousFunctionExpression) Type() Type {
	return AnyType
}

func (*AnonymousFunctionExpression) isExpression() {}

// BinaryOpExpression represents a semantically-analyzed binary operation.
type BinaryOpExpression struct {
	// The syntax node associated with the binary operation.
	Syntax *hclsyntax.BinaryOpExpr

	// The left-hand operand of the operation.
	LeftOperand Expression
	// The right-hand operand of the operation.
	RightOperand Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the binary operation.
func (x *BinaryOpExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the binary operation.
func (x *BinaryOpExpression) Type() Type {
	return x.exprType
}

func (*BinaryOpExpression) isExpression() {}

// ConditionalExpression represents a semantically-analzed conditional expression (i.e.
// <condition> '?' <true> ':' <false>).
type ConditionalExpression struct {
	// The syntax node associated with the conditional expression.
	Syntax *hclsyntax.ConditionalExpr

	// The condition.
	Condition Expression
	// The result of the expression if the condition evaluates to true.
	TrueResult Expression
	// The result of the expression if the condition evaluates to false.
	FalseResult Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the conditional expression.
func (x *ConditionalExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the conditional expression.
func (x *ConditionalExpression) Type() Type {
	return x.exprType
}

func (*ConditionalExpression) isExpression() {}

// ErrorExpression represents an expression that could not be bound due to an error.
type ErrorExpression struct {
	// The syntax node associated with the error,
	Syntax hclsyntax.Node

	exprType Type
}

// SyntaxNode returns the syntax node associated with the error expression.
func (x *ErrorExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the error expression.
func (x *ErrorExpression) Type() Type {
	return x.exprType
}

func (*ErrorExpression) isExpression() {}

// ForExpression represents a semantically-analyzed for expression.
type ForExpression struct {
	// The syntax node associated with the for expression.
	Syntax *hclsyntax.ForExpr

	// The collection being iterated.
	Collection Expression
	// The expression that generates the keys of the result, if any. If this field is non-nil, the result is a map.
	Key Expression
	// The expression that generates the values of the result.
	Value Expression
	// The condition that filters the items of the result, if any.
	Condition Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the for expression.
func (x *ForExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the for expression.
func (x *ForExpression) Type() Type {
	return x.exprType
}

func (*ForExpression) isExpression() {}

// FunctionCallExpression represents a semantically-analyzed function call expression.
type FunctionCallExpression struct {
	// The syntax node associated with the function call expression.
	Syntax *hclsyntax.FunctionCallExpr

	// The name of the called function.
	Name string
	// The signature of the called function.
	Signature StaticFunctionSignature
	// The arguments to the function call.
	Args []Expression
}

// SyntaxNode returns the syntax node associated with the function call expression.
func (x *FunctionCallExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the function call expression.
func (x *FunctionCallExpression) Type() Type {
	return x.Signature.ReturnType
}

func (*FunctionCallExpression) isExpression() {}

// IndexExpression represents a semantically-analyzed index expression.
type IndexExpression struct {
	// The syntax node associated with the index expression.
	Syntax *hclsyntax.IndexExpr

	// The collection being indexed.
	Collection Expression
	// The index key.
	Key Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the index expression.
func (x *IndexExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the index expression.
func (x *IndexExpression) Type() Type {
	return x.exprType
}

func (*IndexExpression) isExpression() {}

// LiteralValueExpression represents a semantically-analyzed literal value expression.
type LiteralValueExpression struct {
	// The syntax node associated with the literal value expression.
	Syntax *hclsyntax.LiteralValueExpr

	// The value of the expression.
	Value resource.PropertyValue

	exprType Type
}

// SyntaxNode returns the syntax node associated with the literal value expression.
func (x *LiteralValueExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the literal value expression.
func (x *LiteralValueExpression) Type() Type {
	return x.exprType
}

func (*LiteralValueExpression) isExpression() {}

// ObjectConsExpression represents a semantically-analyzed object construction expression.
type ObjectConsExpression struct {
	// The syntax node associated with the object construction expression.
	Syntax *hclsyntax.ObjectConsExpr

	// The items that comprise the object construction expression.
	Items []ObjectConsItem

	exprType Type
}

// SyntaxNode returns the syntax node associated with the object construction expression.
func (x *ObjectConsExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the object construction expression.
func (x *ObjectConsExpression) Type() Type {
	return x.exprType
}

// ObjectConsItem records a key-value pair that is part of object construction expression.
type ObjectConsItem struct {
	// The key.
	Key Expression
	// The value.
	Value Expression
}

func (*ObjectConsExpression) isExpression() {}

// RelativeTraversalExpression represents a semantically-analyzed relative traversal expression.
type RelativeTraversalExpression struct {
	// The syntax node associated with the relative traversal expression.
	Syntax *hclsyntax.RelativeTraversalExpr

	// The expression that computes the value being traversed.
	Source Expression
	// The traversal's parts.
	Parts []Traversable
}

// SyntaxNode returns the syntax node associated with the relative traversal expression.
func (x *RelativeTraversalExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the relative traversal expression.
func (x *RelativeTraversalExpression) Type() Type {
	return GetTraversableType(x.Parts[len(x.Parts)-1])
}

func (*RelativeTraversalExpression) isExpression() {}

// ScopeTraversalExpression represents a semantically-analyzed scope traversal expression.
type ScopeTraversalExpression struct {
	// The syntax node associated with the scope traversal expression.
	Syntax *hclsyntax.ScopeTraversalExpr

	// The traversal's parts.
	Parts []Traversable
}

// SyntaxNode returns the syntax node associated with the scope traversal expression.
func (x *ScopeTraversalExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the scope traversal expression.
func (x *ScopeTraversalExpression) Type() Type {
	return GetTraversableType(x.Parts[len(x.Parts)-1])
}

func (*ScopeTraversalExpression) isExpression() {}

// SplatExpression represents a semantically-analyzed splat expression.
type SplatExpression struct {
	// The syntax node associated with the splat expression.
	Syntax *hclsyntax.SplatExpr

	// The expression being splatted.
	Source Expression
	// The expression applied to each element of the splat.
	Each Expression
	// The local variable definition associated with the current item being processed. This definition is not part of
	// a scope, and can only be referenced by an AnonSymbolExpr.
	Item *Variable

	exprType Type
}

// SyntaxNode returns the syntax node associated with the splat expression.
func (x *SplatExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the splat expression.
func (x *SplatExpression) Type() Type {
	return x.exprType
}

func (*SplatExpression) isExpression() {}

// TemplateExpression represents a semantically-analyzed template expression.
type TemplateExpression struct {
	// The syntax node associated with the template expression.
	Syntax *hclsyntax.TemplateExpr

	// The parts of the template expression.
	Parts []Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the template expression.
func (x *TemplateExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the template expression.
func (x *TemplateExpression) Type() Type {
	return x.exprType
}

func (*TemplateExpression) isExpression() {}

// TemplateJoinExpression represents a semantically-analyzed template join expression.
type TemplateJoinExpression struct {
	// The syntax node associated with the template join expression.
	Syntax *hclsyntax.TemplateJoinExpr

	// The tuple being joined.
	Tuple Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the template join expression.
func (x *TemplateJoinExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the template join expression.
func (x *TemplateJoinExpression) Type() Type {
	return x.exprType
}

func (*TemplateJoinExpression) isExpression() {}

// TupleConsExpression represents a semantically-analyzed tuple construction expression.
type TupleConsExpression struct {
	// The syntax node associated with the tuple construction expression.
	Syntax *hclsyntax.TupleConsExpr

	// The elements of the tuple.
	Expressions []Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the tuple construction expression.
func (x *TupleConsExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the tuple construction expression.
func (x *TupleConsExpression) Type() Type {
	return x.exprType
}

func (*TupleConsExpression) isExpression() {}

// UnaryOpExpression represents a semantically-analyzed unary operation.
type UnaryOpExpression struct {
	// The syntax node associated with the unary operation.
	Syntax *hclsyntax.UnaryOpExpr

	// The operand of the operation.
	Operand Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the unary operation.
func (x *UnaryOpExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// Type returns the type of the unary operation.
func (x *UnaryOpExpression) Type() Type {
	return x.exprType
}

func (*UnaryOpExpression) isExpression() {}
