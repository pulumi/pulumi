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
	"fmt"
	"io"
	"math/big"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
	"github.com/zclconf/go-cty/cty"
)

// Expression represents a semantically-analyzed HCL2 expression.
type Expression interface {
	printable

	// SyntaxNode returns the hclsyntax.Node associated with the expression.
	SyntaxNode() hclsyntax.Node
	// NodeTokens returns the syntax.Tokens associated with the expression.
	NodeTokens() syntax.NodeTokens

	// GetLeadingTrivia returns the leading trivia associated with the expression.
	GetLeadingTrivia() syntax.TriviaList
	// SetLeadingTrivia sets the leading trivia associated with the expression.
	SetLeadingTrivia(syntax.TriviaList)
	// GetTrailingTrivia returns the trailing trivia associated with the expression.
	GetTrailingTrivia() syntax.TriviaList
	// SetTrailingTrivia sets the trailing trivia associated with the expression.
	SetTrailingTrivia(syntax.TriviaList)

	// Type returns the type of the expression.
	Type() Type

	isExpression()
}

func identToken(token syntax.Token, ident string) syntax.Token {
	if string(token.Raw.Bytes) != ident {
		token.Raw.Bytes = []byte(ident)
	}
	return token
}

func exprHasLeadingTrivia(parens syntax.Parentheses, first interface{}) bool {
	if parens.Any() {
		return true
	}
	switch first := first.(type) {
	case Expression:
		return first.HasLeadingTrivia()
	case syntax.NodeTokens:
		return true
	}
	return false
}

func exprHasTrailingTrivia(parens syntax.Parentheses, last interface{}) bool {
	if parens.Any() {
		return true
	}
	switch last := last.(type) {
	case Expression:
		return last.HasTrailingTrivia()
	case syntax.NodeTokens:
		return true
	}
	return false
}

func getExprLeadingTrivia(parens syntax.Parentheses, first interface{}) syntax.TriviaList {
	if parens.Any() {
		return parens.GetLeadingTrivia()
	}
	switch first := first.(type) {
	case Expression:
		return first.GetLeadingTrivia()
	case syntax.Token:
		return first.LeadingTrivia
	}
	return nil
}

func setExprLeadingTrivia(parens syntax.Parentheses, first interface{}, trivia syntax.TriviaList) {
	if parens.Any() {
		parens.SetLeadingTrivia(trivia)
		return
	}
	switch first := first.(type) {
	case Expression:
		first.SetLeadingTrivia(trivia)
	case *syntax.Token:
		first.LeadingTrivia = trivia
	}
}

func getExprTrailingTrivia(parens syntax.Parentheses, last interface{}) syntax.TriviaList {
	if parens.Any() {
		return parens.GetTrailingTrivia()
	}
	switch last := last.(type) {
	case Expression:
		return last.GetTrailingTrivia()
	case syntax.Token:
		return last.TrailingTrivia
	}
	return nil
}

func setExprTrailingTrivia(parens syntax.Parentheses, last interface{}, trivia syntax.TriviaList) {
	if parens.Any() {
		parens.SetTrailingTrivia(trivia)
		return
	}
	switch last := last.(type) {
	case Expression:
		last.SetTrailingTrivia(trivia)
	case *syntax.Token:
		last.TrailingTrivia = trivia
	}
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

// NodeTokens returns the tokens associated with the body of the anonymous function.
func (x *AnonymousFunctionExpression) NodeTokens() syntax.NodeTokens {
	return x.Body.NodeTokens()
}

// Type returns the type of the anonymous function expression.
//
// TODO: currently this returns the any type. Instead, it should return a function type.
func (x *AnonymousFunctionExpression) Type() Type {
	return DynamicType
}

func (x *AnonymousFunctionExpression) HasLeadingTrivia() bool {
	return x.Body.HasLeadingTrivia()
}

func (x *AnonymousFunctionExpression) HasTrailingTrivia() bool {
	return x.Body.HasTrailingTrivia()
}

func (x *AnonymousFunctionExpression) GetLeadingTrivia() syntax.TriviaList {
	return x.Body.GetLeadingTrivia()
}

func (x *AnonymousFunctionExpression) SetLeadingTrivia(t syntax.TriviaList) {
	x.Body.SetLeadingTrivia(t)
}

func (x *AnonymousFunctionExpression) GetTrailingTrivia() syntax.TriviaList {
	return x.Body.GetTrailingTrivia()
}

func (x *AnonymousFunctionExpression) SetTrailingTrivia(t syntax.TriviaList) {
	x.Body.SetTrailingTrivia(t)
}

func (x *AnonymousFunctionExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *AnonymousFunctionExpression) print(w io.Writer, p *printer) {
	// Print a call to eval.
	p.fprintf(w, "eval(")

	// Print the parameter names.
	for _, v := range x.Parameters {
		p.fprintf(w, "%v, ", v.Name)
	}

	// Print the body and closing paren.
	p.fprintf(w, "%v)", x.Body)
}

func (*AnonymousFunctionExpression) isExpression() {}

// BinaryOpExpression represents a semantically-analyzed binary operation.
type BinaryOpExpression struct {
	// The syntax node associated with the binary operation.
	Syntax *hclsyntax.BinaryOpExpr
	// The tokens associated with the expression, if any.
	Tokens *syntax.BinaryOpTokens

	// The left-hand operand of the operation.
	LeftOperand Expression
	// The operation.
	Operation *hclsyntax.Operation
	// The right-hand operand of the operation.
	RightOperand Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the binary operation.
func (x *BinaryOpExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the binary operation.
func (x *BinaryOpExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

// Type returns the type of the binary operation.
func (x *BinaryOpExpression) Type() Type {
	return x.exprType
}

func (x *BinaryOpExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.LeftOperand)
}

func (x *BinaryOpExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.RightOperand)
}

func (x *BinaryOpExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.LeftOperand)
}

func (x *BinaryOpExpression) SetLeadingTrivia(t syntax.TriviaList) {
	setExprLeadingTrivia(x.Tokens.GetParentheses(), x.LeftOperand, t)
}

func (x *BinaryOpExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.RightOperand)
}

func (x *BinaryOpExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewBinaryOpTokens(x.Operation)
	}
	setExprTrailingTrivia(x.Tokens.GetParentheses(), x.RightOperand, t)
}

func (x *BinaryOpExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *BinaryOpExpression) print(w io.Writer, p *printer) {
	p.fprintf(w, "%(%v% v% v%)",
		x.Tokens.Parentheses,
		x.LeftOperand, x.Tokens.GetOperator(x.Operation), x.RightOperand,
		x.Tokens.Parentheses)
}

func (*BinaryOpExpression) isExpression() {}

// ConditionalExpression represents a semantically-analzed conditional expression (i.e.
// <condition> '?' <true> ':' <false>).
type ConditionalExpression struct {
	// The syntax node associated with the conditional expression.
	Syntax *hclsyntax.ConditionalExpr
	// The tokens associated with the expression, if any.
	Tokens syntax.NodeTokens

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

// NodeTokens returns the tokens associated with the conditional expression.
func (x *ConditionalExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

// Type returns the type of the conditional expression.
func (x *ConditionalExpression) Type() Type {
	return x.exprType
}

func (x *ConditionalExpression) HasLeadingTrivia() bool {
	switch tokens := x.Tokens.(type) {
	case *syntax.ConditionalTokens:
		if tokens.Parentheses.Any() {
			return true
		}
	case *syntax.TemplateConditionalTokens:
		return len(tokens.OpenIf.LeadingTrivia) != 0
	}
	return x.Condition.HasLeadingTrivia()
}

func (x *ConditionalExpression) HasTrailingTrivia() bool {
	switch tokens := x.Tokens.(type) {
	case *syntax.ConditionalTokens:
		if tokens.Parentheses.Any() {
			return true
		}
	case *syntax.TemplateConditionalTokens:
		return len(tokens.CloseEndif.TrailingTrivia) != 0
	}
	return x.FalseResult.HasTrailingTrivia()
}

func (x *ConditionalExpression) GetLeadingTrivia() syntax.TriviaList {
	switch tokens := x.Tokens.(type) {
	case *syntax.ConditionalTokens:
		if tokens.Parentheses.Any() {
			return tokens.Parentheses.GetLeadingTrivia()
		}
	case *syntax.TemplateConditionalTokens:
		return tokens.OpenIf.LeadingTrivia
	}
	return x.Condition.GetLeadingTrivia()
}

func (x *ConditionalExpression) SetLeadingTrivia(t syntax.TriviaList) {
	switch tokens := x.Tokens.(type) {
	case *syntax.ConditionalTokens:
		if tokens.Parentheses.Any() {
			tokens.Parentheses.SetLeadingTrivia(t)
			return
		}
	case *syntax.TemplateConditionalTokens:
		tokens.OpenIf.LeadingTrivia = t
		return
	}
	x.Condition.SetLeadingTrivia(t)
}

func (x *ConditionalExpression) GetTrailingTrivia() syntax.TriviaList {
	switch tokens := x.Tokens.(type) {
	case *syntax.ConditionalTokens:
		if tokens.Parentheses.Any() {
			return tokens.Parentheses.GetTrailingTrivia()
		}
	case *syntax.TemplateConditionalTokens:
		return tokens.CloseEndif.TrailingTrivia
	}
	return x.FalseResult.GetTrailingTrivia()
}

func (x *ConditionalExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewConditionalTokens()
	}
	switch tokens := x.Tokens.(type) {
	case *syntax.ConditionalTokens:
		if tokens.Parentheses.Any() {
			tokens.Parentheses.SetTrailingTrivia(t)
			return
		}
	case *syntax.TemplateConditionalTokens:
		tokens.CloseEndif.TrailingTrivia = t
		return
	}
	x.FalseResult.SetTrailingTrivia(t)
}

func (x *ConditionalExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *ConditionalExpression) print(w io.Writer, p *printer) {
	tokens := x.Tokens
	if tokens == nil {
		tokens = syntax.NewConditionalTokens()
	}

	switch tokens := tokens.(type) {
	case *syntax.ConditionalTokens:
		p.fprintf(w, "%(%v% v% v% v% v%)",
			tokens.Parentheses,
			x.Condition, tokens.QuestionMark, x.TrueResult, tokens.Colon, x.FalseResult,
			tokens.Parentheses)
	case *syntax.TemplateConditionalTokens:
		p.fprintf(w, "%v%v% v%v%v", tokens.OpenIf, tokens.If, x.Condition, tokens.CloseIf, x.TrueResult)
		if tokens.Else != nil {
			p.fprintf(w, "%v%v%v%v", tokens.OpenElse, tokens.Else, tokens.CloseElse, x.FalseResult)
		}
		p.fprintf(w, "%v%v%v", tokens.OpenEndif, tokens.Endif, tokens.CloseEndif)
	}
}

func (*ConditionalExpression) isExpression() {}

// ErrorExpression represents an expression that could not be bound due to an error.
type ErrorExpression struct {
	// The syntax node associated with the error, if any.
	Syntax hclsyntax.Node
	// The tokens associated with the error.
	Tokens syntax.NodeTokens
	// The message associated with the error.
	Message string

	exprType Type
}

// SyntaxNode returns the syntax node associated with the error expression.
func (x *ErrorExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the error expression.
func (x *ErrorExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

// Type returns the type of the error expression.
func (x *ErrorExpression) Type() Type {
	return x.exprType
}

func (x *ErrorExpression) HasLeadingTrivia() bool {
	return false
}

func (x *ErrorExpression) HasTrailingTrivia() bool {
	return false
}

func (x *ErrorExpression) GetLeadingTrivia() syntax.TriviaList {
	return nil
}

func (x *ErrorExpression) SetLeadingTrivia(t syntax.TriviaList) {
}

func (x *ErrorExpression) GetTrailingTrivia() syntax.TriviaList {
	return nil
}

func (x *ErrorExpression) SetTrailingTrivia(t syntax.TriviaList) {
}

func (x *ErrorExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *ErrorExpression) print(w io.Writer, p *printer) {
	p.fprintf(w, "error(%q)", x.Message)
}

func (*ErrorExpression) isExpression() {}

// ForExpression represents a semantically-analyzed for expression.
type ForExpression struct {
	// The syntax node associated with the for expression.
	Syntax *hclsyntax.ForExpr
	// The tokens associated with the expression, if any.
	Tokens syntax.NodeTokens

	// The key variable, if any.
	KeyVariable *Variable
	// The value variable.
	ValueVariable *Variable

	// The collection being iterated.
	Collection Expression
	// The expression that generates the keys of the result, if any. If this field is non-nil, the result is a map.
	Key Expression
	// The expression that generates the values of the result.
	Value Expression
	// The condition that filters the items of the result, if any.
	Condition Expression

	// True if the value expression is being grouped.
	Group bool

	exprType Type
}

// SyntaxNode returns the syntax node associated with the for expression.
func (x *ForExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the for expression.
func (x *ForExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

// Type returns the type of the for expression.
func (x *ForExpression) Type() Type {
	return x.exprType
}

func (x *ForExpression) HasLeadingTrivia() bool {
	return x.Tokens != nil
}

func (x *ForExpression) HasTrailingTrivia() bool {
	return x.Tokens != nil
}

func (x *ForExpression) GetLeadingTrivia() syntax.TriviaList {
	switch tokens := x.Tokens.(type) {
	case *syntax.ForTokens:
		return getExprLeadingTrivia(tokens.Parentheses, tokens.Open)
	case *syntax.TemplateForTokens:
		return tokens.OpenFor.LeadingTrivia
	default:
		return nil
	}
}

func (x *ForExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		keyVariable := ""
		if x.KeyVariable != nil {
			keyVariable = x.KeyVariable.Name
		}
		x.Tokens = syntax.NewForTokens(keyVariable, x.ValueVariable.Name, x.Key != nil, x.Group, x.Condition != nil)
	}
	switch tokens := x.Tokens.(type) {
	case *syntax.ForTokens:
		setExprLeadingTrivia(tokens.Parentheses, &tokens.Open, t)
	case *syntax.TemplateForTokens:
		tokens.OpenFor.LeadingTrivia = t
	}
}

func (x *ForExpression) GetTrailingTrivia() syntax.TriviaList {
	switch tokens := x.Tokens.(type) {
	case *syntax.ForTokens:
		return getExprTrailingTrivia(tokens.Parentheses, tokens.Close)
	case *syntax.TemplateForTokens:
		return tokens.CloseEndfor.TrailingTrivia
	default:
		return nil
	}
}

func (x *ForExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		keyVariable := ""
		if x.KeyVariable != nil {
			keyVariable = x.KeyVariable.Name
		}
		x.Tokens = syntax.NewForTokens(keyVariable, x.ValueVariable.Name, x.Key != nil, x.Group, x.Condition != nil)
	}
	switch tokens := x.Tokens.(type) {
	case *syntax.ForTokens:
		setExprTrailingTrivia(tokens.Parentheses, &tokens.Close, t)
	case *syntax.TemplateForTokens:
		tokens.CloseEndfor.TrailingTrivia = t
	}
}

func (x *ForExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *ForExpression) print(w io.Writer, p *printer) {
	tokens := x.Tokens
	if tokens == nil {
		keyVariable := ""
		if x.KeyVariable != nil {
			keyVariable = x.KeyVariable.Name
		}
		syntax.NewForTokens(keyVariable, x.ValueVariable.Name, x.Key != nil, x.Group, x.Condition != nil)
	}

	switch tokens := tokens.(type) {
	case *syntax.ForTokens:
		// Print the opening rune and the for token.
		p.fprintf(w, "%(%v%v", tokens.Parentheses, tokens.Open, tokens.For)

		// Print the key variable, if any.
		if x.KeyVariable != nil {
			keyToken := tokens.Key
			if x.KeyVariable != nil && keyToken == nil {
				keyToken = &syntax.Token{Raw: hclsyntax.Token{Type: hclsyntax.TokenIdent}}
			}

			key := identToken(*keyToken, x.KeyVariable.Name)
			p.fprintf(w, "% v%v", key, tokens.Comma)
		}

		// Print the value variable, the in token, the collection expression, and the colon.
		value := identToken(tokens.Value, x.ValueVariable.Name)
		p.fprintf(w, "% v% v% v%v", value, tokens.In, x.Collection, tokens.Colon)

		// Print the key expression and arrow token, if any.
		if x.Key != nil {
			p.fprintf(w, "% v% v", x.Key, tokens.Arrow)
		}

		// Print the value expression.
		p.fprintf(w, "% v", x.Value)

		// Print the group token, if any.
		if x.Group {
			p.fprintf(w, "%v", tokens.Group)
		}

		// Print the if token and the condition, if any.
		if x.Condition != nil {
			p.fprintf(w, "% v% v", tokens.If, x.Condition)
		}

		// Print the closing rune.
		p.fprintf(w, "%v%)", tokens.Close, tokens.Parentheses)
	case *syntax.TemplateForTokens:
		// Print the opening sequence.
		p.fprintf(w, "%v%v", tokens.OpenFor, tokens.For)

		// Print the key variable, if any.
		if x.KeyVariable != nil {
			keyToken := tokens.Key
			if x.KeyVariable != nil && keyToken == nil {
				keyToken = &syntax.Token{Raw: hclsyntax.Token{Type: hclsyntax.TokenIdent}}
			}

			key := identToken(*keyToken, x.KeyVariable.Name)
			p.fprintf(w, "% v%v", key, tokens.Comma)
		}

		// Print the value variable, the in token, the collection expression, the control sequence terminator, the
		// value expression, and the closing sequence.
		p.fprintf(w, "% v% v% v%v%v%v%v%v",
			identToken(tokens.Value, x.ValueVariable.Name), tokens.In, x.Collection, tokens.CloseFor,
			x.Value,
			tokens.OpenEndfor, tokens.Endfor, tokens.CloseEndfor)
	}
}

func (*ForExpression) isExpression() {}

// FunctionCallExpression represents a semantically-analyzed function call expression.
type FunctionCallExpression struct {
	// The syntax node associated with the function call expression.
	Syntax *hclsyntax.FunctionCallExpr
	// The tokens associated with the expression, if any.
	Tokens *syntax.FunctionCallTokens

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

// NodeTokens returns the tokens associated with the function call expression.
func (x *FunctionCallExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *FunctionCallExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *FunctionCallExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *FunctionCallExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetName(x.Name))
}

func (x *FunctionCallExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewFunctionCallTokens(x.Name, len(x.Args))
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.Name, t)
}

func (x *FunctionCallExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetCloseParen())
}

func (x *FunctionCallExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewFunctionCallTokens(x.Name, len(x.Args))
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, &x.Tokens.CloseParen, t)
}

func (x *FunctionCallExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *FunctionCallExpression) print(w io.Writer, p *printer) {
	// Print the name and opening parenthesis.
	p.fprintf(w, "%(%v%v", x.Tokens.GetParentheses(), x.Tokens.GetName(x.Name), x.Tokens.GetOpenParen())

	// Print each argument and its comma.
	commas := x.Tokens.GetCommas(len(x.Args))
	for i, arg := range x.Args {
		if i == 0 {
			p.fprintf(w, "%v", arg)
		} else {
			p.fprintf(w, "% v", arg)
		}

		if i < len(x.Args)-1 {
			var comma syntax.Token
			if i < len(commas) {
				comma = commas[i]
			}
			p.fprintf(w, "%v", comma)
		}
	}

	// If there were commas left over, print the trivia for each.
	if len(x.Args) > 0 && len(x.Args)-1 <= len(commas) {
		for _, comma := range commas[len(x.Args)-1:] {
			p.fprintf(w, "%v", comma.AllTrivia().CollapseWhitespace())
		}
	}

	// Print the closing parenthesis.
	p.fprintf(w, "%v%)", x.Tokens.GetCloseParen(), x.Tokens.GetParentheses())
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
	// The tokens associated with the expression, if any.
	Tokens *syntax.IndexTokens

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

// NodeTokens returns the tokens associated with the index expression.
func (x *IndexExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *IndexExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Collection)
}

func (x *IndexExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *IndexExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetOpenBracket())
}

func (x *IndexExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewIndexTokens()
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.OpenBracket, t)
}

func (x *IndexExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetCloseBracket())
}

func (x *IndexExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewIndexTokens()
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, &x.Tokens.CloseBracket, t)
}

func (x *IndexExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *IndexExpression) print(w io.Writer, p *printer) {
	p.fprintf(w, "%(%v%v%v%v%)",
		x.Tokens.GetParentheses(),
		x.Collection, x.Tokens.GetOpenBracket(), x.Key, x.Tokens.GetCloseBracket(),
		x.Tokens.GetParentheses())
}

// Type returns the type of the index expression.
func (x *IndexExpression) Type() Type {
	return x.exprType
}

func (*IndexExpression) isExpression() {}

func literalText(value cty.Value, rawBytes []byte) string {
	if len(rawBytes) > 0 {
		parsed, diags := hclsyntax.ParseExpression(rawBytes, "", hcl.Pos{})
		if !diags.HasErrors() {
			if lit, ok := parsed.(*hclsyntax.LiteralValueExpr); ok && lit.Val.RawEquals(value) {
				return string(rawBytes)
			}
		}
	}

	switch value.Type() {
	case cty.Bool:
		if value.True() {
			return "true"
		}
		return "false"
	case cty.Number:
		bf := value.AsBigFloat()
		i, acc := bf.Int64()
		if acc == big.Exact {
			return fmt.Sprintf("%v", i)
		}
		d, _ := bf.Float64()
		return fmt.Sprintf("%g", d)
	case cty.String:
		return value.AsString()
	default:
		panic(fmt.Errorf("unexpected literal type %v", value.Type().FriendlyName()))
	}
}

// LiteralValueExpression represents a semantically-analyzed literal value expression.
type LiteralValueExpression struct {
	// The syntax node associated with the literal value expression.
	Syntax *hclsyntax.LiteralValueExpr
	// The tokens associated with the expression, if any.
	Tokens *syntax.LiteralValueTokens

	// The value of the expression.
	Value cty.Value

	exprType Type
}

// SyntaxNode returns the syntax node associated with the literal value expression.
func (x *LiteralValueExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the literal value expression.
func (x *LiteralValueExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *LiteralValueExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *LiteralValueExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *LiteralValueExpression) GetLeadingTrivia() syntax.TriviaList {
	if v := x.Tokens.GetValue(x.Value); len(v) > 0 {
		return getExprLeadingTrivia(x.Tokens.GetParentheses(), v[0])
	}
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), nil)
}

func (x *LiteralValueExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewLiteralValueTokens(x.Value)
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.Value[0], t)
}

func (x *LiteralValueExpression) GetTrailingTrivia() syntax.TriviaList {
	if v := x.Tokens.GetValue(x.Value); len(v) > 0 {
		return getExprTrailingTrivia(x.Tokens.GetParentheses(), v[len(v)-1])
	}
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), nil)
}

func (x *LiteralValueExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewLiteralValueTokens(x.Value)
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, &x.Tokens.Value[len(x.Tokens.Value)-1], t)
}

func (x *LiteralValueExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *LiteralValueExpression) print(w io.Writer, p *printer) {
	// Literals are... odd. They may be composed of multiple tokens, but those tokens should never contain interior
	// trivia.

	var leading, trailing syntax.TriviaList
	var rawBytes []byte
	if toks := x.Tokens.GetValue(x.Value); len(toks) > 0 {
		leading, trailing = toks[0].LeadingTrivia, toks[len(toks)-1].TrailingTrivia

		for _, t := range toks {
			rawBytes = append(rawBytes, t.Raw.Bytes...)
		}
	}

	p.fprintf(w, "%(%v%v%v%)",
		x.Tokens.GetParentheses(),
		leading, literalText(x.Value, rawBytes), trailing,
		x.Tokens.GetParentheses())
}

// Type returns the type of the literal value expression.
func (x *LiteralValueExpression) Type() Type {
	return x.exprType
}

func (*LiteralValueExpression) isExpression() {}

// ObjectConsItem records a key-value pair that is part of object construction expression.
type ObjectConsItem struct {
	// The key.
	Key Expression
	// The value.
	Value Expression
}

// ObjectConsExpression represents a semantically-analyzed object construction expression.
type ObjectConsExpression struct {
	// The syntax node associated with the object construction expression.
	Syntax *hclsyntax.ObjectConsExpr
	// The tokens associated with the expression, if any.
	Tokens *syntax.ObjectConsTokens

	// The items that comprise the object construction expression.
	Items []ObjectConsItem

	exprType Type
}

// SyntaxNode returns the syntax node associated with the object construction expression.
func (x *ObjectConsExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the object construction expression.
func (x *ObjectConsExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *ObjectConsExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *ObjectConsExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *ObjectConsExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetOpenBrace(len(x.Items)))
}

func (x *ObjectConsExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewObjectConsTokens(len(x.Items))
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.OpenBrace, t)
}

func (x *ObjectConsExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetCloseBrace())
}

func (x *ObjectConsExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewObjectConsTokens(len(x.Items))
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, &x.Tokens.CloseBrace, t)
}

func (x *ObjectConsExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *ObjectConsExpression) print(w io.Writer, p *printer) {
	// Print the opening brace.
	p.fprintf(w, "%(%v", x.Tokens.GetParentheses(), x.Tokens.GetOpenBrace(len(x.Items)))

	// Print the items.
	p.indented(func() {
		items := x.Tokens.GetItems(len(x.Items))
		for i, item := range x.Items {
			tokens := syntax.NewObjectConsItemTokens(i == len(x.Items)-1)
			if i < len(items) {
				tokens = items[i]
			}

			if !item.Key.HasLeadingTrivia() {
				p.fprintf(w, "\n%s", p.indent)
			}
			p.fprintf(w, "%v% v% v", item.Key, tokens.Equals, item.Value)

			if tokens.Comma != nil {
				p.fprintf(w, "%v", tokens.Comma)
			}
		}

		if len(x.Items) < len(items) {
			for _, item := range items[len(x.Items):] {
				p.fprintf(w, "%v", item.Equals.AllTrivia().CollapseWhitespace())
				if item.Comma != nil {
					p.fprintf(w, "%v", item.Comma.AllTrivia().CollapseWhitespace())
				}
			}
		}
	})

	if x.Tokens != nil {
		p.fprintf(w, "%v%)", x.Tokens.CloseBrace, x.Tokens.Parentheses)
	} else {
		p.fprintf(w, "\n%s}", p.indent)
	}
}

// Type returns the type of the object construction expression.
func (x *ObjectConsExpression) Type() Type {
	return x.exprType
}

func (*ObjectConsExpression) isExpression() {}

func traverserHasTrailingTrivia(tokens syntax.TraverserTokens) bool {
	switch tokens := tokens.(type) {
	case *syntax.DotTraverserTokens:
		return exprHasTrailingTrivia(tokens.Parentheses, tokens.Index)
	case *syntax.BracketTraverserTokens:
		return exprHasTrailingTrivia(tokens.Parentheses, tokens.CloseBracket)
	default:
		panic(fmt.Errorf("unexpected traverser of type %T", tokens))
	}
}

func getTraverserTrivia(tokens syntax.TraverserTokens) (syntax.TriviaList, syntax.TriviaList) {
	var leading, trailing syntax.TriviaList
	switch tokens := tokens.(type) {
	case *syntax.DotTraverserTokens:
		leading = getExprLeadingTrivia(tokens.Parentheses, tokens.Dot)
		trailing = getExprTrailingTrivia(tokens.Parentheses, tokens.Index)
	case *syntax.BracketTraverserTokens:
		leading = getExprLeadingTrivia(tokens.Parentheses, tokens.OpenBracket)
		trailing = getExprTrailingTrivia(tokens.Parentheses, tokens.CloseBracket)
	}
	return leading, trailing
}

func setTraverserTrailingTrivia(tokens syntax.TraverserTokens, t syntax.TriviaList) {
	switch tokens := tokens.(type) {
	case *syntax.DotTraverserTokens:
		setExprTrailingTrivia(tokens.Parentheses, &tokens.Index, t)
	case *syntax.BracketTraverserTokens:
		setExprTrailingTrivia(tokens.Parentheses, &tokens.CloseBracket, t)
	default:
		panic(fmt.Errorf("unexpected traverser of type %T", tokens))
	}
}

func printTraverser(w io.Writer, p *printer, t hcl.Traverser, tokens syntax.TraverserTokens) {
	var index string
	switch t := t.(type) {
	case hcl.TraverseAttr:
		index = t.Name
	case hcl.TraverseIndex:
		index = literalText(t.Key, nil)
	default:
		panic(fmt.Errorf("unexpected traverser of type %T", t))
	}

	switch tokens := tokens.(type) {
	case *syntax.DotTraverserTokens:
		p.fprintf(w, "%(%v%v%)",
			tokens.Parentheses,
			tokens.Dot, identToken(tokens.Index, index),
			tokens.Parentheses)
	case *syntax.BracketTraverserTokens:
		p.fprintf(w, "%(%v%v%v%)",
			tokens.Parentheses,
			tokens.OpenBracket, identToken(tokens.Index, index), tokens.CloseBracket,
			tokens.Parentheses)
	default:
		panic(fmt.Errorf("unexpected traverser tokens of type %T", tokens))
	}
}

func printRelativeTraversal(w io.Writer, p *printer, traversal hcl.Traversal, tokens []syntax.TraverserTokens) {
	for i, traverser := range traversal {
		// Fetch the traversal tokens.
		var traverserTokens syntax.TraverserTokens
		if i < len(tokens) {
			traverserTokens = tokens[i]
		}
		printTraverser(w, p, traverser, traverserTokens)
	}

	// Print any remaining trivia.
	if len(traversal) < len(tokens) {
		for _, tokens := range tokens[len(traversal):] {
			var trivia syntax.TriviaList
			switch tokens := tokens.(type) {
			case *syntax.DotTraverserTokens:
				trivia = tokens.Dot.LeadingTrivia
				trivia = append(trivia, tokens.Dot.TrailingTrivia...)
				trivia = append(trivia, tokens.Index.LeadingTrivia...)
				trivia = append(trivia, tokens.Index.TrailingTrivia...)
			case *syntax.BracketTraverserTokens:
				trivia = tokens.OpenBracket.LeadingTrivia
				trivia = append(trivia, tokens.OpenBracket.TrailingTrivia...)
				trivia = append(trivia, tokens.Index.LeadingTrivia...)
				trivia = append(trivia, tokens.Index.TrailingTrivia...)
				trivia = append(trivia, tokens.CloseBracket.LeadingTrivia...)
				trivia = append(trivia, tokens.CloseBracket.TrailingTrivia...)
			}
			p.fprintf(w, "%v", trivia)
		}
	}
}

// RelativeTraversalExpression represents a semantically-analyzed relative traversal expression.
type RelativeTraversalExpression struct {
	// The syntax node associated with the relative traversal expression.
	Syntax *hclsyntax.RelativeTraversalExpr
	// The tokens associated with the expression, if any.
	Tokens *syntax.RelativeTraversalTokens

	// The expression that computes the value being traversed.
	Source Expression
	// The traversal's parts.
	Parts []Traversable

	// The traversers.
	Traversal hcl.Traversal
}

// SyntaxNode returns the syntax node associated with the relative traversal expression.
func (x *RelativeTraversalExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the relative traversal expression.
func (x *RelativeTraversalExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *RelativeTraversalExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Source)
}

func (x *RelativeTraversalExpression) HasTrailingTrivia() bool {
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		return true
	}
	if traversal := x.Tokens.GetTraversal(x.Traversal); len(traversal) > 0 {
		return traverserHasTrailingTrivia(traversal[len(traversal)-1])
	}
	return x.Source.HasTrailingTrivia()
}

func (x *RelativeTraversalExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Source)
}

func (x *RelativeTraversalExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewRelativeTraversalTokens(x.Traversal)
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, x.Source, t)
}

func (x *RelativeTraversalExpression) GetTrailingTrivia() syntax.TriviaList {
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		return parens.GetTrailingTrivia()
	}
	if traversal := x.Tokens.GetTraversal(x.Traversal); len(traversal) > 0 {
		_, trailingTrivia := getTraverserTrivia(traversal[len(traversal)-1])
		return trailingTrivia
	}
	return x.Source.GetTrailingTrivia()
}

func (x *RelativeTraversalExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewRelativeTraversalTokens(x.Traversal)
	}
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		parens.SetTrailingTrivia(t)
		return
	}
	if len(x.Tokens.Traversal) > 0 {
		setTraverserTrailingTrivia(x.Tokens.Traversal[len(x.Tokens.Traversal)-1], t)
		return
	}
	x.Source.SetTrailingTrivia(t)
}

func (x *RelativeTraversalExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *RelativeTraversalExpression) print(w io.Writer, p *printer) {
	// Print the source expression.
	p.fprintf(w, "%(%v", x.Tokens.GetParentheses(), x.Source)

	// Print the traversal.
	printRelativeTraversal(w, p, x.Traversal, x.Tokens.GetTraversal(x.Traversal))

	// Print the closing parentheses, if any.
	p.fprintf(w, "%)", x.Tokens.GetParentheses())
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
	// The tokens associated with the expression, if any.
	Tokens *syntax.ScopeTraversalTokens

	// The traversal's parts.
	Parts []Traversable

	// The root name.
	RootName string
	// The traversers.
	Traversal hcl.Traversal
}

// SyntaxNode returns the syntax node associated with the scope traversal expression.
func (x *ScopeTraversalExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the scope traversal expression.
func (x *ScopeTraversalExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *ScopeTraversalExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *ScopeTraversalExpression) HasTrailingTrivia() bool {
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		return true
	}
	if traversal := x.Tokens.GetTraversal(x.Traversal); len(traversal) > 0 {
		return traverserHasTrailingTrivia(traversal[len(traversal)-1])
	}
	return x.Tokens != nil
}

func (x *ScopeTraversalExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetRoot(x.Traversal))
}

func (x *ScopeTraversalExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewScopeTraversalTokens(x.Traversal)
	}
	x.Tokens.Root.LeadingTrivia = t
}

func (x *ScopeTraversalExpression) GetTrailingTrivia() syntax.TriviaList {
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		return parens.GetTrailingTrivia()
	}
	if traversal := x.Tokens.GetTraversal(x.Traversal); len(traversal) > 0 {
		_, trailingTrivia := getTraverserTrivia(traversal[len(traversal)-1])
		return trailingTrivia
	}
	return x.Tokens.GetRoot(x.Traversal).TrailingTrivia
}

func (x *ScopeTraversalExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewScopeTraversalTokens(x.Traversal)
	}
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		parens.SetTrailingTrivia(t)
		return
	}
	if len(x.Tokens.Traversal) > 0 {
		setTraverserTrailingTrivia(x.Tokens.Traversal[len(x.Tokens.Traversal)-1], t)
		return
	}
	x.Tokens.Root.TrailingTrivia = t
}

func (x *ScopeTraversalExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *ScopeTraversalExpression) print(w io.Writer, p *printer) {
	// Print the root name.
	p.fprintf(w, "%(%v", x.Tokens.GetParentheses(), x.Tokens.GetRoot(x.Traversal))

	// Print the traversal.
	printRelativeTraversal(w, p, x.Traversal[1:], x.Tokens.GetTraversal(x.Traversal))

	// Print the closing parentheses, if any.
	p.fprintf(w, "%)", x.Tokens.GetParentheses())
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
	// The tokens associated with the expression, if any.
	Tokens *syntax.SplatTokens

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

// NodeTokens returns the tokens associated with the splat expression.
func (x *SplatExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *SplatExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Source)
}

func (x *SplatExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Each)
}

func (x *SplatExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Source)
}

func (x *SplatExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewSplatTokens(false)
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.Open, t)
}

func (x *SplatExpression) GetTrailingTrivia() syntax.TriviaList {
	if parens := x.Tokens.GetParentheses(); parens.Any() {
		return parens.GetTrailingTrivia()
	}
	if close := x.Tokens.GetClose(); close != nil {
		return close.TrailingTrivia
	}
	return x.Tokens.GetStar().TrailingTrivia
}

func (x *SplatExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewSplatTokens(false)
	}
	if x.Tokens.Parentheses.Any() {
		x.Tokens.Parentheses.SetTrailingTrivia(t)
		return
	}
	if x.Tokens.Close == nil {
		x.Tokens.Star.TrailingTrivia = t
		return
	}
	x.Tokens.Close.TrailingTrivia = t
}

func (x *SplatExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *SplatExpression) print(w io.Writer, p *printer) {
	isDot := x.Tokens.GetClose() == nil

	p.fprintf(w, "%(%v%v%v", x.Tokens.GetParentheses(), x.Source, x.Tokens.GetOpen(), x.Tokens.GetStar())
	if !isDot {
		p.fprintf(w, "%v", x.Tokens.GetClose())
	}
	p.fprintf(w, "%v%)", x.Each, x.Tokens.GetParentheses())
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
	// The tokens associated with the expression, if any.
	Tokens *syntax.TemplateTokens

	// The parts of the template expression.
	Parts []Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the template expression.
func (x *TemplateExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the template expression.
func (x *TemplateExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *TemplateExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *TemplateExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *TemplateExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetOpen())
}

func (x *TemplateExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewTemplateTokens()
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.Open, t)
}

func (x *TemplateExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetClose())
}

func (x *TemplateExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewTemplateTokens()
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, &x.Tokens.Close, t)
}

func (x *TemplateExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *TemplateExpression) print(w io.Writer, p *printer) {
	// Print the opening quote.
	p.fprintf(w, "%(%v", x.Tokens.GetParentheses(), x.Tokens.GetOpen())

	// Print the expressions.
	for _, part := range x.Parts {
		p.fprintf(w, "%v", part)
	}

	// Print the closing quote
	p.fprintf(w, "%v%)", x.Tokens.GetClose(), x.Tokens.GetParentheses())
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

// NodeTokens returns the tokens associated with the template join expression.
func (x *TemplateJoinExpression) NodeTokens() syntax.NodeTokens {
	return nil
}

func (x *TemplateJoinExpression) HasLeadingTrivia() bool {
	return x.Tuple.HasLeadingTrivia()
}

func (x *TemplateJoinExpression) HasTrailingTrivia() bool {
	return x.Tuple.HasTrailingTrivia()
}

func (x *TemplateJoinExpression) GetLeadingTrivia() syntax.TriviaList {
	return x.Tuple.GetLeadingTrivia()
}

func (x *TemplateJoinExpression) SetLeadingTrivia(t syntax.TriviaList) {
	x.Tuple.SetLeadingTrivia(t)
}

func (x *TemplateJoinExpression) GetTrailingTrivia() syntax.TriviaList {
	return x.Tuple.GetTrailingTrivia()
}

func (x *TemplateJoinExpression) SetTrailingTrivia(t syntax.TriviaList) {
	x.Tuple.SetTrailingTrivia(t)
}

func (x *TemplateJoinExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *TemplateJoinExpression) print(w io.Writer, p *printer) {
	p.fprintf(w, "%v", x.Tuple)
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
	// The tokens associated with the expression, if any.
	Tokens *syntax.TupleConsTokens

	// The elements of the tuple.
	Expressions []Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the tuple construction expression.
func (x *TupleConsExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the tuple construction expression.
func (x *TupleConsExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *TupleConsExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *TupleConsExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens)
}

func (x *TupleConsExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetOpenBracket())
}

func (x *TupleConsExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewTupleConsTokens(len(x.Expressions))
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.OpenBracket, t)
}

func (x *TupleConsExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetCloseBracket())
}

func (x *TupleConsExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewTupleConsTokens(len(x.Expressions))
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, &x.Tokens.CloseBracket, t)
}

func (x *TupleConsExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *TupleConsExpression) print(w io.Writer, p *printer) {
	// Print the opening bracket.
	p.fprintf(w, "%(%v", x.Tokens.GetParentheses(), x.Tokens.GetOpenBracket())

	// Print each element and its comma.
	commas := x.Tokens.GetCommas(len(x.Expressions))
	p.indented(func() {
		for i, expr := range x.Expressions {
			if !expr.HasLeadingTrivia() {
				p.fprintf(w, "\n%s", p.indent)
			}
			p.fprintf(w, "%v", expr)

			if i != len(x.Expressions)-1 {
				var comma syntax.Token
				if i < len(commas) {
					comma = commas[i]
				}
				p.fprintf(w, "%v", comma)
			}
		}

		// If there were commas left over, print the trivia for each.
		//
		// TODO(pdg): filter to only comments?
		if len(x.Expressions) > 0 && len(x.Expressions)-1 <= len(commas) {
			for _, comma := range commas[len(x.Expressions)-1:] {
				p.fprintf(w, "%v", comma.AllTrivia().CollapseWhitespace())
			}
		}
	})

	// Print the closing bracket.
	if x.Tokens != nil {
		p.fprintf(w, "%v%)", x.Tokens.CloseBracket, x.Tokens.GetParentheses())
	} else {
		p.fprintf(w, "\n%s]", p.indent)
	}
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
	// The tokens associated with the expression, if any.
	Tokens *syntax.UnaryOpTokens

	// The operation.
	Operation *hclsyntax.Operation
	// The operand of the operation.
	Operand Expression

	exprType Type
}

// SyntaxNode returns the syntax node associated with the unary operation.
func (x *UnaryOpExpression) SyntaxNode() hclsyntax.Node {
	return x.Syntax
}

// NodeTokens returns the tokens associated with the unary operation.
func (x *UnaryOpExpression) NodeTokens() syntax.NodeTokens {
	return x.Tokens
}

func (x *UnaryOpExpression) HasLeadingTrivia() bool {
	return exprHasLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetOperator(x.Operation))
}

func (x *UnaryOpExpression) HasTrailingTrivia() bool {
	return exprHasTrailingTrivia(x.Tokens.GetParentheses(), x.Operand)
}

func (x *UnaryOpExpression) GetLeadingTrivia() syntax.TriviaList {
	return getExprLeadingTrivia(x.Tokens.GetParentheses(), x.Tokens.GetOperator(x.Operation))
}

func (x *UnaryOpExpression) SetLeadingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewUnaryOpTokens(x.Operation)
	}
	setExprLeadingTrivia(x.Tokens.Parentheses, &x.Tokens.Operator, t)
}

func (x *UnaryOpExpression) GetTrailingTrivia() syntax.TriviaList {
	return getExprTrailingTrivia(x.Tokens.GetParentheses(), x.Operand)
}

func (x *UnaryOpExpression) SetTrailingTrivia(t syntax.TriviaList) {
	if x.Tokens == nil {
		x.Tokens = syntax.NewUnaryOpTokens(x.Operation)
	}
	setExprTrailingTrivia(x.Tokens.Parentheses, x.Operand, t)
}

func (x *UnaryOpExpression) Format(f fmt.State, c rune) {
	x.print(f, &printer{})
}

func (x *UnaryOpExpression) print(w io.Writer, p *printer) {
	p.fprintf(w, "%(%v%v%)",
		x.Tokens.GetParentheses(),
		x.Tokens.GetOperator(x.Operation), x.Operand,
		x.Tokens.GetParentheses())
}

// Type returns the type of the unary operation.
func (x *UnaryOpExpression) Type() Type {
	return x.exprType
}

func (*UnaryOpExpression) isExpression() {}
