package syntax

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func commentString(trivia []Trivia) string {
	s := ""
	for _, t := range trivia {
		if comment, ok := t.(Comment); ok {
			for _, l := range comment.Lines {
				s += strings.Replace(l, "✱", "*", -1)
			}
		}
	}
	return s
}

func validateTokenLeadingTrivia(t *testing.T, token Token) {
	leadingText := commentString(token.LeadingTrivia)
	if !assert.Equal(t, string(token.Raw.Bytes), leadingText) {
		t.Logf("leading trivia mismatch for token @ %v", token.Range())
	}
}

func validateTokenTrailingTrivia(t *testing.T, token Token) {
	trailingText := commentString(token.TrailingTrivia)
	if trailingText != "" && !assert.Equal(t, string(token.Raw.Bytes), trailingText) {
		t.Logf("trailing trivia mismatch for token @ %v", token.Range())
	}
}

func validateTokenTrivia(t *testing.T, token Token) {
	validateTokenLeadingTrivia(t, token)
	validateTokenTrailingTrivia(t, token)
}

func validateTrivia(t *testing.T, tokens ...interface{}) {
	for _, te := range tokens {
		switch te := te.(type) {
		case Token:
			validateTokenTrivia(t, te)
		case *Token:
			if te != nil {
				validateTokenTrivia(t, *te)
			}
		case []Token:
			for _, token := range te {
				validateTokenTrivia(t, token)
			}
		case []ObjectConsItemTokens:
			for _, token := range te {
				validateTrivia(t, token.Equals, token.Comma)
			}
		case []TraverserTokens:
			for _, tt := range te {
				switch token := tt.(type) {
				case *DotTraverserTokens:
					validateTrivia(t, token.Dot, token.Index)
				case *BracketTraverserTokens:
					validateTrivia(t, token.OpenBracket, token.Index, token.CloseBracket)
				}
			}
		}
	}
}

func validateTemplateStringTrivia(t *testing.T, template *hclsyntax.TemplateExpr, n *hclsyntax.LiteralValueExpr,
	tokens *LiteralValueTokens) {

	index := -1
	for i := range template.Parts {
		if template.Parts[i] == n {
			index = i
			break
		}
	}
	assert.NotEqual(t, -1, index)
	assert.Len(t, tokens.Value, 1)

	value := tokens.Value[0]
	if index == 0 {
		assert.Len(t, value.LeadingTrivia, 0)
	} else {
		delim, ok := value.LeadingTrivia[0].(TemplateDelimiter)
		assert.True(t, ok)
		assert.Equal(t, hclsyntax.TokenTemplateSeqEnd, delim.Type)
	}
	if index == len(template.Parts)-1 {
		assert.Len(t, value.TrailingTrivia, 0)
	} else {
		delim, ok := value.TrailingTrivia[0].(TemplateDelimiter)
		assert.True(t, ok)
		assert.Equal(t, hclsyntax.TokenTemplateInterp, delim.Type)
	}
}

type validator struct {
	t      *testing.T
	tokens TokenMap
	stack  []hclsyntax.Node
}

func (v *validator) Enter(n hclsyntax.Node) hcl.Diagnostics {
	switch n := n.(type) {
	case *hclsyntax.Attribute:
		tokens := v.tokens.ForNode(n).(*AttributeTokens)
		validateTrivia(v.t, tokens.Name, tokens.Equals)
	case *hclsyntax.BinaryOpExpr:
		tokens := v.tokens.ForNode(n).(*BinaryOpTokens)
		validateTrivia(v.t, tokens.Operator)
	case *hclsyntax.Block:
		tokens := v.tokens.ForNode(n).(*BlockTokens)
		validateTrivia(v.t, tokens.Type, tokens.Labels, tokens.OpenBrace, tokens.CloseBrace)
	case *hclsyntax.ConditionalExpr:
		tokens := v.tokens.ForNode(n).(*ConditionalTokens)
		validateTrivia(v.t, tokens.QuestionMark, tokens.Colon)
	case *hclsyntax.ForExpr:
		tokens := v.tokens.ForNode(n).(*ForTokens)
		validateTrivia(v.t, tokens.Open, tokens.For, tokens.Key, tokens.Comma, tokens.Value, tokens.In, tokens.Colon,
			tokens.Arrow, tokens.Group, tokens.If, tokens.Close)
	case *hclsyntax.FunctionCallExpr:
		tokens := v.tokens.ForNode(n).(*FunctionCallTokens)
		validateTrivia(v.t, tokens.Name, tokens.OpenParen, tokens.CloseParen)
	case *hclsyntax.IndexExpr:
		tokens := v.tokens.ForNode(n).(*IndexTokens)
		validateTrivia(v.t, tokens.OpenBracket, tokens.CloseBracket)
	case *hclsyntax.LiteralValueExpr:
		template, isTemplateString := (*hclsyntax.TemplateExpr)(nil), false
		if len(v.stack) > 0 && n.Val.Type().Equals(cty.String) {
			template, isTemplateString = v.stack[len(v.stack)-1].(*hclsyntax.TemplateExpr)
		}

		tokens := v.tokens.ForNode(n).(*LiteralValueTokens)
		if isTemplateString {
			validateTemplateStringTrivia(v.t, template, n, tokens)
		} else {
			validateTrivia(v.t, tokens.Value)
		}
	case *hclsyntax.ObjectConsExpr:
		tokens := v.tokens.ForNode(n).(*ObjectConsTokens)
		validateTrivia(v.t, tokens.OpenBrace, tokens.Items, tokens.CloseBrace)
	case *hclsyntax.RelativeTraversalExpr:
		tokens := v.tokens.ForNode(n).(*RelativeTraversalTokens)
		validateTrivia(v.t, tokens.Traversal)
	case *hclsyntax.ScopeTraversalExpr:
		tokens := v.tokens.ForNode(n).(*ScopeTraversalTokens)
		validateTrivia(v.t, tokens.Root, tokens.Traversal)
	case *hclsyntax.SplatExpr:
		tokens := v.tokens.ForNode(n).(*SplatTokens)
		validateTrivia(v.t, tokens.Open, tokens.Star, tokens.Close)
	case *hclsyntax.TemplateExpr:
		tokens := v.tokens.ForNode(n).(*TemplateTokens)

		validateTokenLeadingTrivia(v.t, tokens.Open)
		assert.Equal(v.t, "", commentString(tokens.Open.TrailingTrivia))

		validateTokenTrailingTrivia(v.t, tokens.Close)
		assert.Equal(v.t, "", commentString(tokens.Close.LeadingTrivia))
	case *hclsyntax.TupleConsExpr:
		tokens := v.tokens.ForNode(n).(*TupleConsTokens)
		validateTrivia(v.t, tokens.OpenBracket, tokens.Commas, tokens.CloseBracket)
	case *hclsyntax.UnaryOpExpr:
		tokens := v.tokens.ForNode(n).(*UnaryOpTokens)
		validateTrivia(v.t, tokens.Operator)
	}

	v.stack = append(v.stack, n)
	return nil
}

func (v *validator) Exit(n hclsyntax.Node) hcl.Diagnostics {
	v.stack = v.stack[:len(v.stack)-1]
	return nil
}

func TestComments(t *testing.T) {
	contents, err := ioutil.ReadFile("./testdata/comments_all.hcl")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	parser := NewParser()
	err = parser.ParseFile(bytes.NewReader(contents), "comments_all.hcl")
	assert.NoError(t, err)

	assert.Len(t, parser.Diagnostics, 0)

	f := parser.Files[0]
	diags := hclsyntax.Walk(f.Body, &validator{t: t, tokens: f.Tokens})
	assert.Nil(t, diags)
}
