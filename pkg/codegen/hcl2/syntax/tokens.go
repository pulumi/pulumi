package syntax

import syntax "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/syntax"

// Trivia represents bytes in a source file that are not syntactically meaningful. This includes whitespace and
// comments.
type Trivia = syntax.Trivia

// TriviaList is a list of trivia.
type TriviaList = syntax.TriviaList

// Comment is a piece of trivia that represents a line or block comment in a source file.
type Comment = syntax.Comment

// Whitespace is a piece of trivia that represents a sequence of whitespace characters in a source file.
type Whitespace = syntax.Whitespace

// TemplateDelimiter is a piece of trivia that represents a token that demarcates an interpolation or control sequence
// inside of a template.
type TemplateDelimiter = syntax.TemplateDelimiter

// Token represents an HCL2 syntax token with attached leading trivia.
type Token = syntax.Token

// NodeTokens is a closed interface that is used to represent arbitrary *Tokens types in this package.
type NodeTokens = syntax.NodeTokens

// Parentheses records enclosing parenthesis tokens for expressions.
type Parentheses = syntax.Parentheses

// AttributeTokens records the tokens associated with an *hclsyntax.Attribute.
type AttributeTokens = syntax.AttributeTokens

// BinaryOpTokens records the tokens associated with an *hclsyntax.BinaryOpExpr.
type BinaryOpTokens = syntax.BinaryOpTokens

// BlockTokens records the tokens associated with an *hclsyntax.Block.
type BlockTokens = syntax.BlockTokens

// BodyTokens records the tokens associated with an *hclsyntax.Body.
type BodyTokens = syntax.BodyTokens

// ConditionalTokens records the tokens associated with an *hclsyntax.ConditionalExpr of the form "a ? t : f".
type ConditionalTokens = syntax.ConditionalTokens

// TemplateConditionalTokens records the tokens associated with an *hclsyntax.ConditionalExpr inside a template
// expression.
type TemplateConditionalTokens = syntax.TemplateConditionalTokens

// ForTokens records the tokens associated with an *hclsyntax.ForExpr.
type ForTokens = syntax.ForTokens

// TemplateForTokens records the tokens associated with an *hclsyntax.ForExpr inside a template.
type TemplateForTokens = syntax.TemplateForTokens

// FunctionCallTokens records the tokens associated with an *hclsyntax.FunctionCallExpr.
type FunctionCallTokens = syntax.FunctionCallTokens

// IndexTokens records the tokens associated with an *hclsyntax.IndexExpr.
type IndexTokens = syntax.IndexTokens

// LiteralValueTokens records the tokens associated with an *hclsyntax.LiteralValueExpr.
type LiteralValueTokens = syntax.LiteralValueTokens

// ObjectConsItemTokens records the tokens associated with an hclsyntax.ObjectConsItem.
type ObjectConsItemTokens = syntax.ObjectConsItemTokens

// ObjectConsTokens records the tokens associated with an *hclsyntax.ObjectConsExpr.
type ObjectConsTokens = syntax.ObjectConsTokens

// TraverserTokens is a closed interface implemented by DotTraverserTokens and BracketTraverserTokens
type TraverserTokens = syntax.TraverserTokens

// DotTraverserTokens records the tokens associated with dotted traverser (i.e. '.' <attr>).
type DotTraverserTokens = syntax.DotTraverserTokens

// BracketTraverserTokens records the tokens associated with a bracketed traverser (i.e. '[' <index> ']').
type BracketTraverserTokens = syntax.BracketTraverserTokens

// RelativeTraversalTokens records the tokens associated with an *hclsyntax.RelativeTraversalExpr.
type RelativeTraversalTokens = syntax.RelativeTraversalTokens

// ScopeTraversalTokens records the tokens associated with an *hclsyntax.ScopeTraversalExpr.
type ScopeTraversalTokens = syntax.ScopeTraversalTokens

// SplatTokens records the tokens associated with an *hclsyntax.SplatExpr.
type SplatTokens = syntax.SplatTokens

// TemplateTokens records the tokens associated with an *hclsyntax.TemplateExpr.
type TemplateTokens = syntax.TemplateTokens

// TupleConsTokens records the tokens associated with an *hclsyntax.TupleConsExpr.
type TupleConsTokens = syntax.TupleConsTokens

// UnaryOpTokens records the tokens associated with an *hclsyntax.UnaryOpExpr.
type UnaryOpTokens = syntax.UnaryOpTokens

// NewWhitespace returns a new piece of whitespace trivia with the given contents.
func NewWhitespace(bytes ...byte) Whitespace {
	return syntax.NewWhitespace(bytes...)
}

// NewTemplateDelimiter creates a new TemplateDelimiter value with the given delimiter type. If the token type is not a
// template delimiter, this function will panic.
func NewTemplateDelimiter(typ hclsyntax.TokenType) TemplateDelimiter {
	return syntax.NewTemplateDelimiter(typ)
}

func OperationTokenType(operation *hclsyntax.Operation) hclsyntax.TokenType {
	return syntax.OperationTokenType(operation)
}

func NewAttributeTokens(name string) *AttributeTokens {
	return syntax.NewAttributeTokens(name)
}

func NewBinaryOpTokens(operation *hclsyntax.Operation) *BinaryOpTokens {
	return syntax.NewBinaryOpTokens(operation)
}

func NewBlockTokens(typ string, labels ...string) *BlockTokens {
	return syntax.NewBlockTokens(typ, labels...)
}

func NewConditionalTokens() *ConditionalTokens {
	return syntax.NewConditionalTokens()
}

func NewTemplateConditionalTokens(hasElse bool) *TemplateConditionalTokens {
	return syntax.NewTemplateConditionalTokens(hasElse)
}

func NewForTokens(keyVariable, valueVariable string, mapFor, group, conditional bool) *ForTokens {
	return syntax.NewForTokens(keyVariable, valueVariable, mapFor, group, conditional)
}

func NewTemplateForTokens(keyVariable, valueVariable string) *TemplateForTokens {
	return syntax.NewTemplateForTokens(keyVariable, valueVariable)
}

func NewFunctionCallTokens(name string, argCount int) *FunctionCallTokens {
	return syntax.NewFunctionCallTokens(name, argCount)
}

func NewIndexTokens() *IndexTokens {
	return syntax.NewIndexTokens()
}

func NewLiteralValueTokens(value cty.Value) *LiteralValueTokens {
	return syntax.NewLiteralValueTokens(value)
}

func NewObjectConsItemTokens(last bool) ObjectConsItemTokens {
	return syntax.NewObjectConsItemTokens(last)
}

func NewObjectConsTokens(itemCount int) *ObjectConsTokens {
	return syntax.NewObjectConsTokens(itemCount)
}

func NewTraverserTokens(traverser hcl.Traverser) TraverserTokens {
	return syntax.NewTraverserTokens(traverser)
}

func NewDotTraverserTokens(index string) *DotTraverserTokens {
	return syntax.NewDotTraverserTokens(index)
}

func NewBracketTraverserTokens(index string) *BracketTraverserTokens {
	return syntax.NewBracketTraverserTokens(index)
}

func NewRelativeTraversalTokens(traversal hcl.Traversal) *RelativeTraversalTokens {
	return syntax.NewRelativeTraversalTokens(traversal)
}

func NewScopeTraversalTokens(traversal hcl.Traversal) *ScopeTraversalTokens {
	return syntax.NewScopeTraversalTokens(traversal)
}

func NewSplatTokens(dotted bool) *SplatTokens {
	return syntax.NewSplatTokens(dotted)
}

func NewTemplateTokens() *TemplateTokens {
	return syntax.NewTemplateTokens()
}

func NewTupleConsTokens(elementCount int) *TupleConsTokens {
	return syntax.NewTupleConsTokens(elementCount)
}

func NewUnaryOpTokens(operation *hclsyntax.Operation) *UnaryOpTokens {
	return syntax.NewUnaryOpTokens(operation)
}

