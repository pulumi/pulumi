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
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
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
	tokenMap tokenMap
	tokens   tokenList
	stack    []hclsyntax.Node
}

func (m *tokenMapper) inTemplate() bool {
	if len(m.stack) < 2 {
		return false
	}
	parent := m.stack[len(m.stack)-2]
	switch parent.(type) {
	case *hclsyntax.TemplateExpr, *hclsyntax.TemplateJoinExpr:
		return true
	}
	return false
}

func (m *tokenMapper) inTemplateControl() bool {
	if len(m.stack) < 3 {
		return false
	}
	parent, grandparent := m.stack[len(m.stack)-2], m.stack[len(m.stack)-3]
	switch parent.(type) {
	case *hclsyntax.ConditionalExpr, *hclsyntax.ForExpr:
		switch grandparent.(type) {
		case *hclsyntax.TemplateExpr, *hclsyntax.TemplateJoinExpr:
			return true
		}
	}
	return false
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

func (m *tokenMapper) Enter(n hclsyntax.Node) hcl.Diagnostics {
	switch n := n.(type) {
	case hclsyntax.Attributes, hclsyntax.Blocks, hclsyntax.ChildScope:
		// Do nothing
	default:
		m.stack = append(m.stack, n)
	}
	return nil
}

func (m *tokenMapper) Exit(n hclsyntax.Node) hcl.Diagnostics {
	var nodeTokens NodeTokens
	switch n := n.(type) {
	case *hclsyntax.Attribute:
		nodeTokens = &AttributeTokens{
			Name:   m.tokens.atPos(n.NameRange.Start),
			Equals: m.tokens.atPos(n.EqualsRange.Start),
		}
	case *hclsyntax.BinaryOpExpr:
		nodeTokens = &BinaryOpTokens{
			Operator: m.tokens.atPos(n.LHS.Range().End),
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
		if m.inTemplate() {
			condition := m.tokens.atPos(n.Condition.Range().Start)

			ift := m.tokens.atOffset(condition.Range().Start.Byte - 1)
			openIf := m.tokens.atOffset(ift.Range().Start.Byte - 1)
			closeIf := m.tokens.atPos(n.Condition.Range().End)

			openEndifOrElse := m.tokens.atPos(n.TrueResult.Range().End)
			endifOrElse := m.tokens.atPos(openEndifOrElse.Range().End)
			closeEndifOrElse := m.tokens.atPos(endifOrElse.Range().End)

			var openElse, elset, closeElse *Token
			if endifOrElse.Raw.Type == hclsyntax.TokenIdent && string(endifOrElse.Raw.Bytes) == "else" {
				open, t, close := openEndifOrElse, endifOrElse, closeEndifOrElse
				openElse, elset, closeElse = &open, &t, &close

				openEndifOrElse = m.tokens.atPos(n.FalseResult.Range().End)
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
				QuestionMark: m.tokens.atPos(n.Condition.Range().End),
				Colon:        m.tokens.atPos(n.TrueResult.Range().End),
			}
		}
	case *hclsyntax.ForExpr:
		inTemplate := m.inTemplate()

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
			arrow := m.tokens.atPos(n.KeyExpr.Range().End)
			arrowToken = &arrow
		}

		var groupToken *Token
		if n.Group {
			group := m.tokens.atPos(n.ValExpr.Range().End)
			groupToken = &group
		}

		var ifToken *Token
		if n.CondExpr != nil {
			pos := n.ValExpr.Range().End
			if groupToken != nil {
				pos = groupToken.Range().End
			}
			ift := m.tokens.atPos(pos)
			ifToken = &ift
		}

		if inTemplate {
			closeFor := m.tokens.atPos(n.CollExpr.Range().End)

			openEndfor := m.tokens.atPos(n.ValExpr.Range().End)
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
				Open:  openToken,
				For:   forToken,
				Key:   keyToken,
				Comma: commaToken,
				Value: valueToken,
				In:    m.tokens.atPos(valueToken.Range().End),
				Colon: m.tokens.atPos(n.CollExpr.Range().End),
				Arrow: arrowToken,
				Group: groupToken,
				If:    ifToken,
				Close: m.tokens.atPos(n.CloseRange.Start),
			}
		}
	case *hclsyntax.FunctionCallExpr:
		args := n.Args
		commas := make([]Token, 0, len(args))
		if len(args) > 0 {
			for _, ex := range args[:len(args)-1] {
				commas = append(commas, m.tokens.atPos(ex.Range().End))
			}
			if trailing := m.tokens.atPos(args[len(args)-1].Range().End); trailing.Raw.Type == hclsyntax.TokenComma {
				commas = append(commas, trailing)
			}
		}
		nodeTokens = &FunctionCallTokens{
			Name:       m.tokens.atPos(n.NameRange.Start),
			OpenParen:  m.tokens.atPos(n.OpenParenRange.Start),
			Commas:     commas,
			CloseParen: m.tokens.atPos(n.CloseParenRange.Start),
		}
	case *hclsyntax.IndexExpr:
		nodeTokens = &IndexTokens{
			OpenBracket:  m.tokens.atPos(n.OpenRange.Start),
			CloseBracket: m.tokens.atOffset(n.BracketRange.End.Byte - 1),
		}
	case *hclsyntax.LiteralValueExpr:
		nodeTokens = &LiteralValueTokens{
			Value: m.tokens.inRange(n.Range()),
		}
	case *hclsyntax.ObjectConsKeyExpr:
		nodeTokens = &LiteralValueTokens{
			Value: m.tokens.inRange(n.Range()),
		}
	case *hclsyntax.ObjectConsExpr:
		items := make([]ObjectConsItemTokens, len(n.Items))
		for i, item := range n.Items {
			var comma *Token
			if t := m.tokens.atPos(item.ValueExpr.Range().End); t.Raw.Type == hclsyntax.TokenComma {
				comma = &t
			}
			items[i] = ObjectConsItemTokens{
				Equals: m.tokens.atPos(item.KeyExpr.Range().End),
				Comma:  comma,
			}
		}
		nodeTokens = &ObjectConsTokens{
			OpenBrace:  m.tokens.atPos(n.OpenRange.Start),
			CloseBrace: m.tokens.atOffset(n.SrcRange.End.Byte - 1),
		}
	case *hclsyntax.RelativeTraversalExpr:
		nodeTokens = &RelativeTraversalTokens{
			Traversal: mapRelativeTraversalTokens(m.tokens, n.Traversal),
		}
	case *hclsyntax.ScopeTraversalExpr:
		nodeTokens = &ScopeTraversalTokens{
			Root:      m.tokens.atPos(n.Traversal[0].SourceRange().Start),
			Traversal: mapRelativeTraversalTokens(m.tokens, n.Traversal[1:]),
		}
	case *hclsyntax.SplatExpr:
		openToken := m.tokens.atOffset(n.MarkerRange.Start.Byte - 1)
		starToken := m.tokens.atPos(n.MarkerRange.Start)
		var closeToken *Token
		if openToken.Raw.Type == hclsyntax.TokenOBrack {
			cbrack := m.tokens.atPos(n.MarkerRange.End)
			closeToken = &cbrack
		}
		nodeTokens = &SplatTokens{
			Open:  openToken,
			Star:  starToken,
			Close: closeToken,
		}
	case *hclsyntax.TemplateExpr:
		// NOTE: the HCL parser lifts control sequences into if or for expressions. This is handled in the correspoding
		// mappers.
		if !m.inTemplateControl() {
			// If we're inside the body of a template if or for, there cannot be any delimiting tokens.
			nodeTokens = &TemplateTokens{
				Open:  m.tokens.atPos(n.SrcRange.Start),
				Close: m.tokens.atOffset(n.SrcRange.End.Byte - 1),
			}
		}
	case *hclsyntax.TupleConsExpr:
		exprs := n.Exprs
		commas := make([]Token, 0, len(exprs))
		if len(exprs) > 0 {
			for _, ex := range exprs[:len(exprs)-1] {
				commas = append(commas, m.tokens.atPos(ex.Range().End))
			}
			if trailing := m.tokens.atPos(exprs[len(exprs)-1].Range().End); trailing.Raw.Type == hclsyntax.TokenComma {
				commas = append(commas, trailing)
			}
		}
		nodeTokens = &TupleConsTokens{
			OpenBracket:  m.tokens.atPos(n.OpenRange.Start),
			Commas:       commas,
			CloseBracket: m.tokens.atOffset(n.SrcRange.End.Byte - 1),
		}
	case *hclsyntax.UnaryOpExpr:
		nodeTokens = &UnaryOpTokens{
			Operator: m.tokens.atPos(n.SymbolRange.Start),
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
	var trivia TriviaList
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
					tokens[len(tokens)-1].TrailingTrivia, trivia = trivia, nil

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
				tokens[len(tokens)-1].TrailingTrivia, trivia = trivia, nil
			}
		case hclsyntax.TokenTemplateControl:
			tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), nil
			inControlSeq = true
		case hclsyntax.TokenTemplateSeqEnd:
			// If this terminates a template control sequence, it is a proper token. Otherwise, it is treated as leading
			// trivia.
			if !inControlSeq {
				trivia = append(trivia, TemplateDelimiter{Type: raw.Type, rng: raw.Range, bytes: raw.Bytes})
			} else {
				tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), nil
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
			tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), nil
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
		tokenMap: tokenMap,
		tokens:   tokens,
	})
	contract.Assert(diags == nil)

	// If the root was a Body and there is a trailing end-of-file token, attach it to the body.
	body, isBody := root.(*hclsyntax.Body)
	if isBody && len(tokens) > 0 && tokens[len(tokens)-1].Raw.Type == hclsyntax.TokenEOF {
		tokenMap[body] = &BodyTokens{EndOfFile: &tokens[len(tokens)-1]}
	}
}

func mapRelativeTraversalTokens(tokens tokenList, traversal hcl.Traversal) []TraverserTokens {
	if len(traversal) == 0 {
		return nil
	}

	contract.Assert(traversal.IsRelative())
	items := make([]TraverserTokens, len(traversal))
	for i, t := range traversal {
		rng := t.SourceRange()
		leadingToken := tokens.atPos(rng.Start)
		indexToken := tokens.atOffset(rng.Start.Byte + 1)
		if leadingToken.Raw.Type == hclsyntax.TokenOBrack {
			items[i] = &BracketTraverserTokens{
				OpenBracket:  leadingToken,
				Index:        indexToken,
				CloseBracket: tokens.atOffset(rng.End.Byte - 1),
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
