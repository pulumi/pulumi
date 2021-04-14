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

package syntax

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// tokenList is a list of Tokens with methods to aid in mapping source positions to tokens.
type tokenList []Token

// offsetIndex returns the index of the token that contains the given byte offset or -1 if no such token exists.
func (l tokenList) offsetIndex(offset int) int {
	base := 0
	for len(l) > 0 {
		i := len(l) / 2
		r := l[i].Range()
		switch {
		case offset < r.Start.Byte:
			l = l[:i]
		case r.Start.Byte <= offset && offset < r.End.Byte:
			return base + i
		case r.End.Byte <= offset:
			l, base = l[i+1:], base+i+1
		default:
			contract.Failf("unexpected index condition: %v, %v, %v", r.Start.Byte, r.End.Byte, offset)
		}
	}
	return -1
}

// atOffset returns the token that contains the given byte offset or the zero value if no such token exists.
func (l tokenList) atOffset(offset int) Token {
	if i := l.offsetIndex(offset); i >= 0 {
		return l[i]
	}
	return Token{}
}

// atPos returns the token that contains the given hcl.Pos or the zero value if no such token exists.
func (l tokenList) atPos(p hcl.Pos) Token {
	return l.atOffset(p.Byte)
}

// inRange returns a slice of the tokens that cover the given range or nil if either the start or end position is
// uncovered by a token.
func (l tokenList) inRange(r hcl.Range) []Token {
	// If the range is empty, ignore it.
	if r.Empty() {
		return nil
	}

	// Find the index of the start and end tokens for this range.
	start, end := l.offsetIndex(r.Start.Byte), l.offsetIndex(r.End.Byte-1)
	if start == -1 || end == -1 {
		return nil
	}
	return l[start : end+1]
}

// A TokenMap is used to map from syntax nodes to information about their tokens and leading whitespace/comments.
type TokenMap interface {
	ForNode(n hclsyntax.Node) NodeTokens

	isTokenMap()
}

type tokenMap map[hclsyntax.Node]NodeTokens

// ForNode returns the token information for the given node, if any.
func (m tokenMap) ForNode(n hclsyntax.Node) NodeTokens {
	return m[n]
}

func (tokenMap) isTokenMap() {}

// NewTokenMapForFiles creates a new token map that can be used to look up tokens for nodes in any of the given files.
func NewTokenMapForFiles(files []*File) TokenMap {
	tokens := tokenMap{}
	for _, f := range files {
		for node, ts := range f.Tokens.(tokenMap) {
			tokens[node] = ts
		}
	}
	return tokens
}

type tokenMapper struct {
	tokenMap             tokenMap
	tokens               tokenList
	stack                []hclsyntax.Node
	templateControlExprs codegen.Set
}

func (m *tokenMapper) getParent() (hclsyntax.Node, bool) {
	if len(m.stack) < 2 {
		return nil, false
	}
	return m.stack[len(m.stack)-2], true
}

func (m *tokenMapper) isSingleFunctionCallArg() bool {
	parent, ok := m.getParent()
	if !ok {
		return false
	}
	call, ok := parent.(*hclsyntax.FunctionCallExpr)
	return ok && len(call.Args) == 1
}

func (m *tokenMapper) inTemplateControl() bool {
	if len(m.stack) < 2 {
		return false
	}
	parent := m.stack[len(m.stack)-2]
	return m.templateControlExprs.Has(parent)
}

func (m *tokenMapper) collectLabelTokens(r hcl.Range) Token {
	tokens := m.tokens.inRange(r)
	if len(tokens) != 3 {
		return m.tokens.atPos(r.Start)
	}
	open := tokens[0]
	if open.Raw.Type != hclsyntax.TokenOQuote || len(open.TrailingTrivia) != 0 {
		return m.tokens.atPos(r.Start)
	}
	str := tokens[1]
	if str.Raw.Type != hclsyntax.TokenQuotedLit || len(str.LeadingTrivia) != 0 || len(str.TrailingTrivia) != 0 {
		return m.tokens.atPos(r.Start)
	}
	close := tokens[2]
	if close.Raw.Type != hclsyntax.TokenCQuote || len(close.LeadingTrivia) != 0 {
		return m.tokens.atPos(r.Start)
	}
	return Token{
		Raw: hclsyntax.Token{
			Type:  hclsyntax.TokenQuotedLit,
			Bytes: append(append(open.Raw.Bytes, str.Raw.Bytes...), close.Raw.Bytes...),
			Range: hcl.Range{
				Filename: open.Raw.Range.Filename,
				Start:    open.Raw.Range.Start,
				End:      close.Raw.Range.End,
			},
		},
		LeadingTrivia:  open.LeadingTrivia,
		TrailingTrivia: close.TrailingTrivia,
	}
}

func (m *tokenMapper) mapRelativeTraversalTokens(traversal hcl.Traversal) []TraverserTokens {
	if len(traversal) == 0 {
		return nil
	}

	contract.Assert(traversal.IsRelative())
	items := make([]TraverserTokens, len(traversal))
	for i, t := range traversal {
		rng := t.SourceRange()
		leadingToken := m.tokens.atPos(rng.Start)
		indexToken := m.tokens.atOffset(rng.Start.Byte + 1)
		if leadingToken.Raw.Type == hclsyntax.TokenOBrack {
			if indexToken.Raw.Type == hclsyntax.TokenOQuote {
				indexToken = m.collectLabelTokens(hcl.Range{
					Filename: rng.Filename,
					Start:    hcl.Pos{Byte: rng.Start.Byte + 1},
					End:      hcl.Pos{Byte: rng.End.Byte - 1},
				})
			}
			items[i] = &BracketTraverserTokens{
				OpenBracket:  leadingToken,
				Index:        indexToken,
				CloseBracket: m.tokens.atOffset(rng.End.Byte - 1),
			}
		} else {
			items[i] = &DotTraverserTokens{
				Dot:   leadingToken,
				Index: indexToken,
			}
		}
	}

	return items
}

func (m *tokenMapper) nodeRange(n hclsyntax.Node) hcl.Range {
	tokens, ok := m.tokenMap[n]
	if !ok {
		return n.Range()
	}

	filename := n.Range().Filename

	var parens Parentheses
	var start, end hcl.Pos
	switch n := n.(type) {
	case *hclsyntax.Attribute:
		tokens := tokens.(*AttributeTokens)
		start, end = tokens.Name.Range().Start, m.nodeRange(n.Expr).End
	case *hclsyntax.BinaryOpExpr:
		tokens := tokens.(*BinaryOpTokens)
		parens = tokens.Parentheses
		start, end = m.nodeRange(n.LHS).Start, m.nodeRange(n.RHS).End
	case *hclsyntax.Block:
		tokens := tokens.(*BlockTokens)
		start, end = tokens.Type.Range().Start, tokens.CloseBrace.Range().End
	case *hclsyntax.ConditionalExpr:
		switch tokens := tokens.(type) {
		case *ConditionalTokens:
			parens = tokens.Parentheses
			start, end = m.nodeRange(n.Condition).Start, m.nodeRange(n.FalseResult).End
		case *TemplateConditionalTokens:
			start, end = tokens.OpenIf.Range().Start, tokens.CloseEndif.Range().End
		}
	case *hclsyntax.ForExpr:
		switch tokens := tokens.(type) {
		case *ForTokens:
			parens = tokens.Parentheses
			start, end = tokens.Open.Range().Start, tokens.Close.Range().End
		case *TemplateForTokens:
			start, end = tokens.OpenFor.Range().Start, tokens.CloseEndfor.Range().End
		}
	case *hclsyntax.FunctionCallExpr:
		tokens := tokens.(*FunctionCallTokens)
		parens = tokens.Parentheses
		start, end = tokens.Name.Range().Start, tokens.CloseParen.Range().End
	case *hclsyntax.IndexExpr:
		tokens := tokens.(*IndexTokens)
		parens = tokens.Parentheses
		start, end = m.nodeRange(n.Collection).Start, tokens.CloseBracket.Range().End
	case *hclsyntax.LiteralValueExpr, *hclsyntax.ObjectConsKeyExpr:
		tokens := tokens.(*LiteralValueTokens)
		parens = tokens.Parentheses
		start, end = tokens.Value[0].Range().Start, tokens.Value[len(tokens.Value)-1].Range().End
	case *hclsyntax.ObjectConsExpr:
		tokens := tokens.(*ObjectConsTokens)
		parens = tokens.Parentheses
		start, end = tokens.OpenBrace.Range().Start, tokens.CloseBrace.Range().End
	case *hclsyntax.RelativeTraversalExpr:
		tokens := tokens.(*RelativeTraversalTokens)
		parens = tokens.Parentheses
		start, end = m.nodeRange(n.Source).Start, tokens.Traversal[len(tokens.Traversal)-1].Range().End
	case *hclsyntax.ScopeTraversalExpr:
		tokens := tokens.(*ScopeTraversalTokens)
		parens = tokens.Parentheses
		start, end = tokens.Root.Range().Start, tokens.Root.Range().End
		if len(tokens.Traversal) > 0 {
			end = tokens.Traversal[len(tokens.Traversal)-1].Range().End
		}
	case *hclsyntax.SplatExpr:
		tokens := tokens.(*SplatTokens)
		parens = tokens.Parentheses
		start, end = m.nodeRange(n.Source).Start, m.nodeRange(n.Each).End
	case *hclsyntax.TemplateExpr:
		tokens := tokens.(*TemplateTokens)
		parens = tokens.Parentheses
		start, end = tokens.Open.Range().Start, tokens.Close.Range().End
	case *hclsyntax.TemplateWrapExpr:
		tokens := tokens.(*TemplateTokens)
		parens = tokens.Parentheses
		start, end = tokens.Open.Range().Start, tokens.Close.Range().End
	case *hclsyntax.TupleConsExpr:
		tokens := tokens.(*TupleConsTokens)
		parens = tokens.Parentheses
		start, end = tokens.OpenBracket.Range().Start, tokens.CloseBracket.Range().End
	case *hclsyntax.UnaryOpExpr:
		tokens := tokens.(*UnaryOpTokens)
		parens = tokens.Parentheses
		start, end = tokens.Operator.Range().Start, m.nodeRange(n.Val).End
	}

	return exprRange(filename, parens, start, end)
}

func (m *tokenMapper) Enter(n hclsyntax.Node) hcl.Diagnostics {
	switch n := n.(type) {
	case hclsyntax.Attributes, hclsyntax.Blocks, hclsyntax.ChildScope:
		// Do nothing
	default:
		m.stack = append(m.stack, n)
	}

	switch n := n.(type) {
	case *hclsyntax.ConditionalExpr:
		open := m.tokens.atPos(n.SrcRange.Start)
		if open.Raw.Type == hclsyntax.TokenTemplateControl {
			m.templateControlExprs.Add(n)
		}
	case *hclsyntax.ForExpr:
		open := m.tokens.atPos(n.OpenRange.Start)
		if open.Raw.Type == hclsyntax.TokenTemplateControl {
			m.templateControlExprs.Add(n)
		}
	}

	// Work around a bug in the HCL syntax library. The walkChildNodes implementation for ObjectConsKeyExpr does not
	// descend into its wrapped expression if the wrapped expression can be interpreted as a keyword even if the
	// syntactical form that _forces_ the wrapped expression to be a non-literal is used.
	if x, ok := n.(*hclsyntax.ObjectConsKeyExpr); ok && x.ForceNonLiteral && hcl.ExprAsKeyword(x.Wrapped) != "" {
		return hclsyntax.Walk(x.Wrapped, m)
	}
	return nil
}

func (m *tokenMapper) Exit(n hclsyntax.Node) hcl.Diagnostics {
	// Gather parentheses.
	var parens Parentheses
	startParens, endParens := n.Range().Start.Byte-1, n.Range().End.Byte
	for {
		open, close := m.tokens.atOffset(startParens), m.tokens.atOffset(endParens)
		if open.Raw.Type != hclsyntax.TokenOParen || close.Raw.Type != hclsyntax.TokenCParen {
			break
		}
		parens.Open, parens.Close = append(parens.Open, open), append(parens.Close, close)
		startParens, endParens = open.Range().Start.Byte-1, close.Range().End.Byte
	}
	if m.isSingleFunctionCallArg() && len(parens.Open) > 0 {
		parens.Open, parens.Close = parens.Open[:len(parens.Open)-1], parens.Close[:len(parens.Close)-1]
	}

	var nodeTokens NodeTokens
	switch n := n.(type) {
	case *hclsyntax.Attribute:
		nodeTokens = &AttributeTokens{
			Name:   m.tokens.atPos(n.NameRange.Start),
			Equals: m.tokens.atPos(n.EqualsRange.Start),
		}
	case *hclsyntax.BinaryOpExpr:
		nodeTokens = &BinaryOpTokens{
			Parentheses: parens,
			Operator:    m.tokens.atPos(m.nodeRange(n.LHS).End),
		}
	case *hclsyntax.Block:
		labels := make([]Token, len(n.Labels))
		for i, r := range n.LabelRanges {
			labels[i] = m.collectLabelTokens(r)
		}
		nodeTokens = &BlockTokens{
			Type:       m.tokens.atPos(n.TypeRange.Start),
			Labels:     labels,
			OpenBrace:  m.tokens.atPos(n.OpenBraceRange.Start),
			CloseBrace: m.tokens.atPos(n.CloseBraceRange.Start),
		}
	case *hclsyntax.ConditionalExpr:
		condRange := m.nodeRange(n.Condition)
		trueRange := m.nodeRange(n.TrueResult)
		falseRange := m.nodeRange(n.FalseResult)

		if m.templateControlExprs.Has(n) {
			condition := m.tokens.atPos(condRange.Start)

			ift := m.tokens.atOffset(condition.Range().Start.Byte - 1)
			openIf := m.tokens.atOffset(ift.Range().Start.Byte - 1)
			closeIf := m.tokens.atPos(condRange.End)

			openEndifOrElse := m.tokens.atPos(trueRange.End)
			endifOrElse := m.tokens.atPos(openEndifOrElse.Range().End)
			closeEndifOrElse := m.tokens.atPos(endifOrElse.Range().End)

			var openElse, elset, closeElse *Token
			if endifOrElse.Raw.Type == hclsyntax.TokenIdent && string(endifOrElse.Raw.Bytes) == "else" {
				open, t, close := openEndifOrElse, endifOrElse, closeEndifOrElse
				openElse, elset, closeElse = &open, &t, &close

				openEndifOrElse = m.tokens.atPos(falseRange.End)
				endifOrElse = m.tokens.atPos(openEndifOrElse.Range().End)
				closeEndifOrElse = m.tokens.atPos(endifOrElse.Range().End)
			}

			nodeTokens = &TemplateConditionalTokens{
				OpenIf:     openIf,
				If:         ift,
				CloseIf:    closeIf,
				OpenElse:   openElse,
				Else:       elset,
				CloseElse:  closeElse,
				OpenEndif:  openEndifOrElse,
				Endif:      endifOrElse,
				CloseEndif: closeEndifOrElse,
			}
		} else {
			nodeTokens = &ConditionalTokens{
				Parentheses:  parens,
				QuestionMark: m.tokens.atPos(condRange.End),
				Colon:        m.tokens.atPos(trueRange.End),
			}
		}
	case *hclsyntax.ForExpr:
		openToken := m.tokens.atPos(n.OpenRange.Start)
		forToken := m.tokens.atPos(openToken.Range().End)

		var keyToken, commaToken *Token
		var valueToken Token
		if n.KeyVar != "" {
			key := m.tokens.atPos(forToken.Range().End)
			comma := m.tokens.atPos(key.Range().End)
			value := m.tokens.atPos(comma.Range().End)

			keyToken, commaToken, valueToken = &key, &comma, value
		} else {
			valueToken = m.tokens.atPos(forToken.Range().End)
		}

		var arrowToken *Token
		if n.KeyExpr != nil {
			arrow := m.tokens.atPos(m.nodeRange(n.KeyExpr).End)
			arrowToken = &arrow
		}

		valRange := m.nodeRange(n.ValExpr)

		var groupToken *Token
		if n.Group {
			group := m.tokens.atPos(valRange.End)
			groupToken = &group
		}

		var ifToken *Token
		if n.CondExpr != nil {
			pos := valRange.End
			if groupToken != nil {
				pos = groupToken.Range().End
			}
			ift := m.tokens.atPos(pos)
			ifToken = &ift
		}

		if m.templateControlExprs.Has(n) {
			closeFor := m.tokens.atPos(m.nodeRange(n.CollExpr).End)

			openEndfor := m.tokens.atPos(valRange.End)
			endfor := m.tokens.atPos(openEndfor.Range().End)
			closeEndfor := m.tokens.atPos(endfor.Range().End)

			nodeTokens = &TemplateForTokens{
				OpenFor:     openToken,
				For:         forToken,
				Key:         keyToken,
				Comma:       commaToken,
				Value:       valueToken,
				In:          m.tokens.atPos(valueToken.Range().End),
				CloseFor:    closeFor,
				OpenEndfor:  openEndfor,
				Endfor:      endfor,
				CloseEndfor: closeEndfor,
			}
		} else {
			nodeTokens = &ForTokens{
				Parentheses: parens,
				Open:        openToken,
				For:         forToken,
				Key:         keyToken,
				Comma:       commaToken,
				Value:       valueToken,
				In:          m.tokens.atPos(valueToken.Range().End),
				Colon:       m.tokens.atPos(m.nodeRange(n.CollExpr).End),
				Arrow:       arrowToken,
				Group:       groupToken,
				If:          ifToken,
				Close:       m.tokens.atPos(n.CloseRange.Start),
			}
		}
	case *hclsyntax.FunctionCallExpr:
		args := n.Args
		commas := make([]Token, 0, len(args))
		if len(args) > 0 {
			for _, ex := range args[:len(args)-1] {
				commas = append(commas, m.tokens.atPos(m.nodeRange(ex).End))
			}
			if trailing := m.tokens.atPos(m.nodeRange(args[len(args)-1]).End); trailing.Raw.Type == hclsyntax.TokenComma {
				commas = append(commas, trailing)
			}
		}
		nodeTokens = &FunctionCallTokens{
			Parentheses: parens,
			Name:        m.tokens.atPos(n.NameRange.Start),
			OpenParen:   m.tokens.atPos(n.OpenParenRange.Start),
			Commas:      commas,
			CloseParen:  m.tokens.atPos(n.CloseParenRange.Start),
		}
	case *hclsyntax.IndexExpr:
		nodeTokens = &IndexTokens{
			Parentheses:  parens,
			OpenBracket:  m.tokens.atPos(n.OpenRange.Start),
			CloseBracket: m.tokens.atOffset(n.BracketRange.End.Byte - 1),
		}
	case *hclsyntax.LiteralValueExpr:
		nodeTokens = &LiteralValueTokens{
			Parentheses: parens,
			Value:       m.tokens.inRange(n.Range()),
		}
	case *hclsyntax.ObjectConsKeyExpr:
		nodeTokens = &LiteralValueTokens{
			Parentheses: parens,
			Value:       m.tokens.inRange(n.Range()),
		}
	case *hclsyntax.ObjectConsExpr:
		items := make([]ObjectConsItemTokens, len(n.Items))
		for i, item := range n.Items {
			var comma *Token
			if t := m.tokens.atPos(m.nodeRange(item.ValueExpr).End); t.Raw.Type == hclsyntax.TokenComma {
				comma = &t
			}
			items[i] = ObjectConsItemTokens{
				Equals: m.tokens.atPos(m.nodeRange(item.KeyExpr).End),
				Comma:  comma,
			}
		}
		nodeTokens = &ObjectConsTokens{
			Parentheses: parens,
			OpenBrace:   m.tokens.atPos(n.OpenRange.Start),
			Items:       items,
			CloseBrace:  m.tokens.atOffset(n.SrcRange.End.Byte - 1),
		}
	case *hclsyntax.RelativeTraversalExpr:
		nodeTokens = &RelativeTraversalTokens{
			Parentheses: parens,
			Traversal:   m.mapRelativeTraversalTokens(n.Traversal),
		}
	case *hclsyntax.ScopeTraversalExpr:
		nodeTokens = &ScopeTraversalTokens{
			Parentheses: parens,
			Root:        m.tokens.atPos(n.Traversal[0].SourceRange().Start),
			Traversal:   m.mapRelativeTraversalTokens(n.Traversal[1:]),
		}
	case *hclsyntax.SplatExpr:
		openToken := m.tokens.atPos(m.nodeRange(n.Source).End)
		starToken := m.tokens.atPos(openToken.Range().End)
		var closeToken *Token
		if openToken.Raw.Type == hclsyntax.TokenOBrack {
			cbrack := m.tokens.atPos(starToken.Range().End)
			closeToken = &cbrack
		}
		nodeTokens = &SplatTokens{
			Parentheses: parens,
			Open:        openToken,
			Star:        starToken,
			Close:       closeToken,
		}
	case *hclsyntax.TemplateExpr:
		// NOTE: the HCL parser lifts control sequences into if or for expressions. This is handled in the correspoding
		// mappers.
		if m.inTemplateControl() {
			nodeTokens = &TemplateTokens{
				Open: Token{
					Raw: hclsyntax.Token{
						Range: hcl.Range{
							Filename: n.SrcRange.Filename,
							Start:    n.SrcRange.Start,
							End:      n.SrcRange.Start,
						},
					},
				},
				Close: Token{
					Raw: hclsyntax.Token{
						Range: hcl.Range{
							Filename: n.SrcRange.Filename,
							Start:    n.SrcRange.End,
							End:      n.SrcRange.End,
						},
					},
				},
			}
		} else {
			// If we're inside the body of a template if or for, there cannot be any delimiting tokens.
			nodeTokens = &TemplateTokens{
				Parentheses: parens,
				Open:        m.tokens.atPos(n.SrcRange.Start),
				Close:       m.tokens.atOffset(n.SrcRange.End.Byte - 1),
			}
		}
	case *hclsyntax.TemplateWrapExpr:
		nodeTokens = &TemplateTokens{
			Parentheses: parens,
			Open:        m.tokens.atPos(n.SrcRange.Start),
			Close:       m.tokens.atOffset(n.SrcRange.End.Byte - 1),
		}
	case *hclsyntax.TupleConsExpr:
		exprs := n.Exprs
		commas := make([]Token, 0, len(exprs))
		if len(exprs) > 0 {
			for _, ex := range exprs[:len(exprs)-1] {
				commas = append(commas, m.tokens.atPos(m.nodeRange(ex).End))
			}
			if trailing := m.tokens.atPos(m.nodeRange(exprs[len(exprs)-1]).End); trailing.Raw.Type == hclsyntax.TokenComma {
				commas = append(commas, trailing)
			}
		}
		nodeTokens = &TupleConsTokens{
			Parentheses:  parens,
			OpenBracket:  m.tokens.atPos(n.OpenRange.Start),
			Commas:       commas,
			CloseBracket: m.tokens.atOffset(n.SrcRange.End.Byte - 1),
		}
	case *hclsyntax.UnaryOpExpr:
		nodeTokens = &UnaryOpTokens{
			Parentheses: parens,
			Operator:    m.tokens.atPos(n.SymbolRange.Start),
		}
	}
	if nodeTokens != nil {
		m.tokenMap[n] = nodeTokens
	}

	if n == m.stack[len(m.stack)-1] {
		m.stack = m.stack[:len(m.stack)-1]
	}

	return nil
}

// mapTokens builds a mapping from the syntax nodes in the given source file to their tokens. The mapping is recorded
// in the map passed in to the function.
func mapTokens(rawTokens hclsyntax.Tokens, filename string, root hclsyntax.Node, contents []byte, tokenMap tokenMap,
	initialPos hcl.Pos) {

	// Turn the list of raw tokens into a list of trivia-carrying tokens.
	lastEndPos := initialPos
	var tokens tokenList
	trivia := TriviaList{}
	inControlSeq := false
	for _, raw := range rawTokens {
		// Snip whitespace out of the body and turn it in to trivia.
		if startPos := raw.Range.Start; startPos.Byte != lastEndPos.Byte {
			triviaBytes := contents[lastEndPos.Byte-initialPos.Byte : startPos.Byte-initialPos.Byte]

			// If this trivia begins a new line, attach the current trivia to the last processed token, if any.
			if len(tokens) > 0 {
				if nl := bytes.IndexByte(triviaBytes, '\n'); nl != -1 {
					trailingTriviaBytes := triviaBytes[:nl+1]
					triviaBytes = triviaBytes[nl+1:]

					endPos := hcl.Pos{Line: lastEndPos.Line + 1, Column: 0, Byte: lastEndPos.Byte + nl + 1}
					rng := hcl.Range{Filename: filename, Start: lastEndPos, End: endPos}
					trivia = append(trivia, Whitespace{rng: rng, bytes: trailingTriviaBytes})
					tokens[len(tokens)-1].TrailingTrivia, trivia = trivia, TriviaList{}

					lastEndPos = endPos
				}
			}

			rng := hcl.Range{Filename: filename, Start: lastEndPos, End: startPos}
			trivia = append(trivia, Whitespace{rng: rng, bytes: triviaBytes})
		}

		switch raw.Type {
		case hclsyntax.TokenComment:
			trivia = append(trivia, Comment{Lines: processComment(raw.Bytes), rng: raw.Range, bytes: raw.Bytes})
		case hclsyntax.TokenTemplateInterp:
			// Treat these as trailing trivia.
			trivia = append(trivia, TemplateDelimiter{Type: raw.Type, rng: raw.Range, bytes: raw.Bytes})
			if len(tokens) > 0 {
				tokens[len(tokens)-1].TrailingTrivia, trivia = trivia, TriviaList{}
			}
		case hclsyntax.TokenTemplateControl:
			tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), TriviaList{}
			inControlSeq = true
		case hclsyntax.TokenTemplateSeqEnd:
			// If this terminates a template control sequence, it is a proper token. Otherwise, it is treated as leading
			// trivia.
			if !inControlSeq {
				tokens[len(tokens)-1].TrailingTrivia = trivia
				trivia = TriviaList{TemplateDelimiter{Type: raw.Type, rng: raw.Range, bytes: raw.Bytes}}
			} else {
				tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), TriviaList{}
			}
			inControlSeq = false
		case hclsyntax.TokenNewline, hclsyntax.TokenBitwiseAnd, hclsyntax.TokenBitwiseOr,
			hclsyntax.TokenBitwiseNot, hclsyntax.TokenBitwiseXor, hclsyntax.TokenStarStar, hclsyntax.TokenApostrophe,
			hclsyntax.TokenBacktick, hclsyntax.TokenSemicolon, hclsyntax.TokenTabs, hclsyntax.TokenInvalid,
			hclsyntax.TokenBadUTF8, hclsyntax.TokenQuotedNewline:
			// Treat these as whitespace. We cannot omit their bytes from the list entirely, as the algorithm below
			// that maps positions to tokens requires that every byte in the source file is covered by the token list.
			continue
		default:
			tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), TriviaList{}
		}
		lastEndPos = raw.Range.End
	}

	// If we had any tokens, we should have attached all trivia to something.
	contract.Assert(len(trivia) == 0 || len(tokens) == 0)

	// Now build the token map.
	//
	// If a node records the ranges of relevant tokens in its syntax node, the start position of those ranges is used
	// to fetch the corresponding token.
	//
	// If a node does not record the range of a relevant token, the end position of a preceding element (e.g. an
	// expression or a token) is used to find the token.
	//
	// TODO(pdg): handle parenthesized expressions
	diags := hclsyntax.Walk(root, &tokenMapper{
		tokenMap:             tokenMap,
		tokens:               tokens,
		templateControlExprs: codegen.Set{},
	})
	contract.Assert(diags == nil)

	// If the root was a Body and there is a trailing end-of-file token, attach it to the body.
	body, isBody := root.(*hclsyntax.Body)
	if isBody && len(tokens) > 0 && tokens[len(tokens)-1].Raw.Type == hclsyntax.TokenEOF {
		tokenMap[body] = &BodyTokens{EndOfFile: &tokens[len(tokens)-1]}
	}
}

// processComment separates the given comment into lines and attempts to remove comment tokens.
func processComment(bytes []byte) []string {
	comment := string(bytes)

	// Each HCL comment may be either a line comment or a block comment. Line comments start with '#' or '//' and
	// terminate with an EOL. Block comments begin with a '/*' and terminate with a '*/'. All comment delimiters are
	// preserved in the HCL comment text.
	//
	// To make life easier for the code generators, HCL comments are pre-processed to remove comment delimiters. For
	// line comments, this process is trivial. For block comments, things are a bit more involved.
	switch {
	case comment[0] == '#':
		return []string{strings.TrimSuffix(comment[1:], "\n")}
	case comment[0:2] == "//":
		return []string{strings.TrimSuffix(comment[2:], "\n")}
	default:
		return processBlockComment(comment)
	}
}

// These regexes are used by processBlockComment. The first matches a block comment start, the second a block comment
// end, and the third a block comment line prefix.
var blockStartPat = regexp.MustCompile(`^/\*+`)
var blockEndPat = regexp.MustCompile(`[[:space:]]*\*+/$`)
var blockPrefixPat = regexp.MustCompile(`^[[:space:]]*\*`)

// processBlockComment splits a block comment into mutiple lines, removes comment delimiters, and attempts to remove
// common comment prefixes from interior lines. For example, the following HCL block comment:
//
//     /**
//      * This is a block comment!
//      *
//      * It has multiple lines.
//      */
//
// becomes this set of lines:
//
//     []string{" This is a block comment!", "", " It has multiple lines."}
//
func processBlockComment(text string) []string {
	lines := strings.Split(text, "\n")

	// We will always trim the following:
	// - '^/\*+' from the first line
	// - a trailing '[[:space:]]\*+/$' from the last line

	// In addition, we will trim '^[[:space:]]*\*' from the second through last lines iff all lines in that set share
	// a prefix that matches that pattern.

	prefix := ""
	for i, l := range lines[1:] {
		m := blockPrefixPat.FindString(l)
		if i == 0 {
			prefix = m
		} else if m != prefix {
			prefix = ""
			break
		}
	}

	for i, l := range lines {
		switch i {
		case 0:
			start := blockStartPat.FindString(l)
			contract.Assert(start != "")
			l = l[len(start):]

			// If this is a single-line block comment, trim the end pattern as well.
			if len(lines) == 1 {
				contract.Assert(prefix == "")

				if end := blockEndPat.FindString(l); end != "" {
					l = l[:len(l)-len(end)]
				}
			}
		case len(lines) - 1:
			// The prefix we're trimming may overlap with the end pattern we're trimming. In this case, trim the entire
			// line.
			if len(l)-len(prefix) == 1 {
				l = ""
			} else {
				l = l[len(prefix):]
				if end := blockEndPat.FindString(l); end != "" {
					l = l[:len(l)-len(end)]
				}
			}
		default:
			// Trim the prefix.
			l = l[len(prefix):]
		}

		lines[i] = l
	}

	// If the first or last line is empty, drop it.
	if lines[0] == "" {
		lines = lines[1:]
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}
