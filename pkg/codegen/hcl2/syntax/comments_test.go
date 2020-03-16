package syntax

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
)

func commentString(trivia []Trivia) string {
	s := ""
	for _, t := range trivia {
		if comment, ok := t.(Comment); ok {
			for _, l := range comment.Lines {
				s += l
			}
		}
	}
	return s
}

func validateTokenTrivia(t *testing.T, token Token) {
	leadingText := commentString(token.LeadingTrivia)
	if leadingText != "TODO" {
		if !assert.Equal(t, string(token.Raw.Bytes), leadingText) {
			t.Logf("leading trivia mismatch for token @ %v", token.Range())
		}
	}

	if trailingText := commentString(token.TrailingTrivia); trailingText != "" && trailingText != "TODO" {
		if !assert.Equal(t, string(token.Raw.Bytes), trailingText) {
			t.Logf("trailing trivia mismatch for token @ %v", token.Range())
		}
	}
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
				case DotTraverserTokens:
					validateTrivia(t, token.Dot, token.Index)
				case BracketTraverserTokens:
					validateTrivia(t, token.OpenBracket, token.Index, token.CloseBracket)
				}
			}
		}
	}
}

type validator struct {
	t          *testing.T
	tokens     TokenMap
	inTemplate bool
}

func (v *validator) Enter(n hclsyntax.Node) hcl.Diagnostics {
	switch n := n.(type) {
	case *hclsyntax.Attribute:
		tokens := v.tokens.ForNode(n).(AttributeTokens)
		validateTrivia(v.t, tokens.Name, tokens.Equals)
	case *hclsyntax.BinaryOpExpr:
		tokens := v.tokens.ForNode(n).(BinaryOpTokens)
		validateTrivia(v.t, tokens.Operator)
	case *hclsyntax.Block:
		tokens := v.tokens.ForNode(n).(BlockTokens)
		validateTrivia(v.t, tokens.Type, tokens.Labels, tokens.OpenBrace, tokens.CloseBrace)
	case *hclsyntax.ConditionalExpr:
		tokens := v.tokens.ForNode(n).(ConditionalTokens)
		validateTrivia(v.t, tokens.QuestionMark, tokens.Colon)
	case *hclsyntax.ForExpr:
		tokens := v.tokens.ForNode(n).(ForTokens)
		validateTrivia(v.t, tokens.Open, tokens.For, tokens.Key, tokens.Comma, tokens.Value, tokens.In, tokens.Colon, tokens.Arrow, tokens.Group, tokens.If, tokens.Close)
	case *hclsyntax.FunctionCallExpr:
		tokens := v.tokens.ForNode(n).(FunctionCallTokens)
		validateTrivia(v.t, tokens.Name, tokens.OpenParen, tokens.CloseParen)
	case *hclsyntax.IndexExpr:
		tokens := v.tokens.ForNode(n).(IndexTokens)
		validateTrivia(v.t, tokens.OpenBracket, tokens.CloseBracket)
	case *hclsyntax.LiteralValueExpr:
		// TODO(pdg): validate string literals
		if !v.inTemplate {
			tokens := v.tokens.ForNode(n).(LiteralValueTokens)
			validateTrivia(v.t, tokens.Value)
		}
	case *hclsyntax.ObjectConsExpr:
		tokens := v.tokens.ForNode(n).(ObjectConsTokens)
		validateTrivia(v.t, tokens.OpenBrace, tokens.Items, tokens.CloseBrace)
	case *hclsyntax.RelativeTraversalExpr:
		tokens := v.tokens.ForNode(n).(RelativeTraversalTokens)
		validateTrivia(v.t, tokens.Traversal)
	case *hclsyntax.ScopeTraversalExpr:
		tokens := v.tokens.ForNode(n).(ScopeTraversalTokens)
		validateTrivia(v.t, tokens.Root, tokens.Traversal)
	case *hclsyntax.SplatExpr:
		tokens := v.tokens.ForNode(n).(SplatTokens)
		validateTrivia(v.t, tokens.Open, tokens.Star, tokens.Close)
	case *hclsyntax.TemplateExpr:
		// TODO(pdg): validate template tokens.
		v.inTemplate = true
	case *hclsyntax.TupleConsExpr:
		tokens := v.tokens.ForNode(n).(TupleConsTokens)
		validateTrivia(v.t, tokens.OpenBracket, tokens.Commas, tokens.CloseBracket)
	case *hclsyntax.UnaryOpExpr:
		tokens := v.tokens.ForNode(n).(UnaryOpTokens)
		validateTrivia(v.t, tokens.Operator)
	}
	return nil
}

func (v *validator) Exit(n hclsyntax.Node) hcl.Diagnostics {
	if _, isTemplate := n.(*hclsyntax.TemplateExpr); isTemplate {
		v.inTemplate = false
	}
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
