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
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(AttributeTokens)
		validateTrivia(v.t, ts.Name, ts.Equals)
	case *hclsyntax.BinaryOpExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(BinaryOpTokens)
		validateTrivia(v.t, ts.Operator)
	case *hclsyntax.Block:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(BlockTokens)
		validateTrivia(v.t, ts.Type, ts.Labels, ts.OpenBrace, ts.CloseBrace)
	case *hclsyntax.ConditionalExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(ConditionalTokens)
		validateTrivia(v.t, ts.QuestionMark, ts.Colon)
	case *hclsyntax.ForExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(ForTokens)
		validateTrivia(v.t, ts.Open, ts.For, ts.Key, ts.Comma, ts.Value, ts.In, ts.Colon, ts.Arrow, ts.Group, ts.If, ts.Close)
	case *hclsyntax.FunctionCallExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(FunctionCallTokens)
		validateTrivia(v.t, ts.Name, ts.OpenParen, ts.CloseParen)
	case *hclsyntax.IndexExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(IndexTokens)
		validateTrivia(v.t, ts.OpenBracket, ts.CloseBracket)
	case *hclsyntax.LiteralValueExpr:
		// TODO(pdg): validate string literals
		if !v.inTemplate {
			tokens, _ := v.tokens.ForNode(n)
			ts := tokens.(LiteralValueTokens)
			validateTrivia(v.t, ts.Value)
		}
	case *hclsyntax.ObjectConsExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(ObjectConsTokens)
		validateTrivia(v.t, ts.OpenBrace, ts.Items, ts.CloseBrace)
	case *hclsyntax.RelativeTraversalExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(RelativeTraversalTokens)
		validateTrivia(v.t, ts.Traversal)
	case *hclsyntax.ScopeTraversalExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(ScopeTraversalTokens)
		validateTrivia(v.t, ts.Root, ts.Traversal)
	case *hclsyntax.SplatExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(SplatTokens)
		validateTrivia(v.t, ts.Open, ts.Star, ts.Close)
	case *hclsyntax.TemplateExpr:
		// TODO(pdg): validate template tokens.
		v.inTemplate = true
	case *hclsyntax.TupleConsExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(TupleConsTokens)
		validateTrivia(v.t, ts.OpenBracket, ts.Commas, ts.CloseBracket)
	case *hclsyntax.UnaryOpExpr:
		tokens, _ := v.tokens.ForNode(n)
		ts := tokens.(UnaryOpTokens)
		validateTrivia(v.t, ts.Operator)
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
