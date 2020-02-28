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
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Trivia represents bytes in a source file that are not syntactically meaningful. This includes whitespace and
// comments.
type Trivia interface {
	// Range returns the range of the trivia in the source file.
	Range() hcl.Range
	// Bytes returns the raw bytes that comprise the trivia.
	Bytes() []byte

	isTrivia()
}

// Comment is a piece of trivia that represents a line or block comment in a source file.
type Comment struct {
	// Lines contains the lines of the comment without leading comment characters or trailing newlines.
	Lines []string

	rng   hcl.Range
	bytes []byte
}

// Range returns the range of the comment in the source file.
func (c Comment) Range() hcl.Range {
	return c.rng
}

// Bytes returns the raw bytes that comprise the comment.
func (c Comment) Bytes() []byte {
	return c.bytes
}

func (Comment) isTrivia() {}

// Whitespace is a piece of trivia that represents a sequence of whitespace characters in a source file.
type Whitespace struct {
	rng   hcl.Range
	bytes []byte
}

// Range returns the range of the whitespace in the source file.
func (w Whitespace) Range() hcl.Range {
	return w.rng
}

// Bytes returns the raw bytes that comprise the whitespace.
func (w Whitespace) Bytes() []byte {
	return w.bytes
}

func (Whitespace) isTrivia() {}

// Token represents an HCL2 syntax token with attached leading trivia.
type Token struct {
	Raw            hclsyntax.Token
	LeadingTrivia  []Trivia
	TrailingTrivia []Trivia
}

// Range returns the total range covered by this token and any leading trivia.
func (t Token) Range() hcl.Range {
	start := t.Raw.Range.Start
	if len(t.LeadingTrivia) > 0 {
		start = t.LeadingTrivia[0].Range().Start
	}
	end := t.Raw.Range.End
	if len(t.TrailingTrivia) > 0 {
		end = t.TrailingTrivia[len(t.TrailingTrivia)-1].Range().End
	}
	return hcl.Range{Filename: t.Raw.Range.Filename, Start: start, End: end}
}

// NodeTokens is a closed interface that is used to represent arbitrary *Tokens types in this package.
type NodeTokens interface {
	isNodeTokens()
}

// AttributeTokens records the tokens associated with an *hclsyntax.Attribute.
type AttributeTokens struct {
	Name   Token
	Equals Token
}

func (AttributeTokens) isNodeTokens() {}

// BinaryOpTokens records the tokens associated with an *hclsyntax.BinaryOpExpr.
type BinaryOpTokens struct {
	Operator Token
}

func (BinaryOpTokens) isNodeTokens() {}

// BlockTokens records the tokens associated with an *hclsyntax.Block.
type BlockTokens struct {
	Type       Token
	Labels     []Token
	OpenBrace  Token
	CloseBrace Token
}

func (BlockTokens) isNodeTokens() {}

// BodyTokens records the tokens associated with an *hclsyntax.Body.
type BodyTokens struct {
	EndOfFile Token
}

func (BodyTokens) isNodeTokens() {}

// ConditionalTokens records the tokens associated with an *hclsyntax.ConditionalExpr.
type ConditionalTokens struct {
	QuestionMark Token
	Colon        Token
}

func (ConditionalTokens) isNodeTokens() {}

// ForTokens records the tokens associated with an *hclsyntax.ForExpr.
type ForTokens struct {
	Open  Token
	For   Token
	Key   *Token
	Comma *Token
	Value Token
	In    Token
	Colon Token
	Arrow *Token
	Group *Token
	If    *Token
	Close Token
}

func (ForTokens) isNodeTokens() {}

// FunctionCallTokens records the tokens associated with an *hclsyntax.FunctionCallExpr.
type FunctionCallTokens struct {
	Name       Token
	OpenParen  Token
	CloseParen Token
}

func (FunctionCallTokens) isNodeTokens() {}

// IndexTokens records the tokens associated with an *hclsyntax.IndexExpr.
type IndexTokens struct {
	OpenBracket  Token
	CloseBracket Token
}

func (IndexTokens) isNodeTokens() {}

// LiteralValueTokens records the tokens associated with an *hclsyntax.LiteralValueExpr.
type LiteralValueTokens struct {
	Value []Token
}

func (LiteralValueTokens) isNodeTokens() {}

// ObjectConsItemTokens records the tokens associated with an hclsyntax.ObjectConsItem.
type ObjectConsItemTokens struct {
	Equals Token
	Comma  *Token
}

// ObjectConsTokens records the tokens associated with an *hclsyntax.ObjectConsExpr.
type ObjectConsTokens struct {
	OpenBrace  Token
	Items      []ObjectConsItemTokens
	CloseBrace Token
}

func (ObjectConsTokens) isNodeTokens() {}

// TraverserTokens is a closed interface implemented by DotTraverserTokens and BracketTraverserTokens
type TraverserTokens interface {
	isTraverserTokens()
}

// DotTraverserTokens records the tokens associated with dotted traverser (i.e. '.' <attr>).
type DotTraverserTokens struct {
	Dot   Token
	Index Token
}

func (DotTraverserTokens) isTraverserTokens() {}

// BracketTraverserTokens records the tokens associated with a bracketed traverser (i.e. '[' <index> ']').
type BracketTraverserTokens struct {
	OpenBracket  Token
	Index        Token
	CloseBracket Token
}

func (BracketTraverserTokens) isTraverserTokens() {}

// RelativeTraversalTokens records the tokens associated with an *hclsyntax.RelativeTraversalExpr.
type RelativeTraversalTokens struct {
	Traversal []TraverserTokens
}

func (RelativeTraversalTokens) isNodeTokens() {}

// ScopeTraversalTokens records the tokens associated with an *hclsyntax.ScopeTraversalExpr.
type ScopeTraversalTokens struct {
	Root      Token
	Traversal []TraverserTokens
}

func (ScopeTraversalTokens) isNodeTokens() {}

// SplatTokens records the tokens associated with an *hclsyntax.SplatExpr.
type SplatTokens struct {
	Open  Token
	Star  Token
	Close *Token
}

func (SplatTokens) isNodeTokens() {}

// TemplateTokens records the tokens associated with an *hclsyntax.TemplateExpr.
type TemplateTokens struct {
	// TODO(pdg): tokens for template parts

	Open  Token
	Close Token
}

func (TemplateTokens) isNodeTokens() {}

// TupleConsTokens records the tokens associated with an *hclsyntax.TupleConsExpr.
type TupleConsTokens struct {
	OpenBracket  Token
	Commas       []Token
	CloseBracket Token
}

func (TupleConsTokens) isNodeTokens() {}

// UnaryOpTokens records the tokens associated with an *hclsyntax.UnaryOpExpr.
type UnaryOpTokens struct {
	Operator Token
}

func (UnaryOpTokens) isNodeTokens() {}

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
			l, base = l[i:], base+i
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
	ForNode(n hclsyntax.Node) (NodeTokens, bool)

	isTokenMap()
}

type tokenMap map[hclsyntax.Node]NodeTokens

// ForNode returns the token information for the given node, if any.
func (m tokenMap) ForNode(n hclsyntax.Node) (NodeTokens, bool) {
	tokens, ok := m[n]
	return tokens, ok
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

// mapTokens builds a mapping from the syntax nodes in the given source file to their tokens. The mapping is recorded
// in the map passed in to the function.
func mapTokens(rawTokens hclsyntax.Tokens, filename string, file *hcl.File, tokenMap tokenMap) {
	// Turn the list of raw tokens into a list of trivia-carrying tokens.
	var lastEndPos hcl.Pos
	var tokens tokenList
	var trivia []Trivia
	for _, raw := range rawTokens {
		// Snip whitespace out of the body and turn it in to trivia.
		if startPos := raw.Range.Start; startPos.Byte != lastEndPos.Byte {
			triviaBytes := file.Bytes[lastEndPos.Byte:startPos.Byte]

			// If this trivia begins a new line, attach the current trivia to the last processed token, if any.
			if len(tokens) > 0 {
				if nl := bytes.IndexByte(triviaBytes, '\n'); nl != -1 {
					trailingTriviaBytes := triviaBytes[:nl+1]
					triviaBytes = trailingTriviaBytes[nl+1:]
					lastEndPos = hcl.Pos{Line: lastEndPos.Line + 1, Column: 0, Byte: lastEndPos.Byte + nl + 1}

					tokens[len(tokens)-1].TrailingTrivia, trivia = trivia, nil
				}
			}

			rng := hcl.Range{Filename: filename, Start: lastEndPos, End: startPos}
			trivia = append(trivia, Whitespace{rng: rng, bytes: triviaBytes})
		}

		switch raw.Type {
		case hclsyntax.TokenComment:
			trivia = append(trivia, Comment{Lines: processComment(raw.Bytes), rng: raw.Range, bytes: raw.Bytes})
			lastEndPos = raw.Range.End
		case hclsyntax.TokenNewline, hclsyntax.TokenBitwiseAnd, hclsyntax.TokenBitwiseOr,
			hclsyntax.TokenBitwiseNot, hclsyntax.TokenBitwiseXor, hclsyntax.TokenStarStar, hclsyntax.TokenApostrophe,
			hclsyntax.TokenBacktick, hclsyntax.TokenSemicolon, hclsyntax.TokenTabs, hclsyntax.TokenInvalid,
			hclsyntax.TokenBadUTF8, hclsyntax.TokenQuotedNewline:
			// Treat them as whitespace. We cannot omit their bytes from the list entirely, as the algorithm below
			// that maps positions to tokens requires that every byte in the source file is covered by the token list.
		default:
			tokens, trivia = append(tokens, Token{Raw: raw, LeadingTrivia: trivia}), nil
			lastEndPos = raw.Range.End
		}
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
	body := file.Body.(*hclsyntax.Body)
	diags := hclsyntax.VisitAll(body, func(n hclsyntax.Node) hcl.Diagnostics {
		var nodeTokens NodeTokens
		switch n := n.(type) {
		case *hclsyntax.Attribute:
			nodeTokens = AttributeTokens{
				Name:   tokens.atPos(n.NameRange.Start),
				Equals: tokens.atPos(n.EqualsRange.Start),
			}
		case *hclsyntax.BinaryOpExpr:
			nodeTokens = BinaryOpTokens{
				Operator: tokens.atPos(n.LHS.Range().End),
			}
		case *hclsyntax.Block:
			labels := make([]Token, len(n.Labels))
			for i, r := range n.LabelRanges {
				labels[i] = tokens.atPos(r.Start)
			}
			nodeTokens = BlockTokens{
				Type:       tokens.atPos(n.TypeRange.Start),
				Labels:     labels,
				OpenBrace:  tokens.atPos(n.OpenBraceRange.Start),
				CloseBrace: tokens.atPos(n.CloseBraceRange.Start),
			}
		case *hclsyntax.ConditionalExpr:
			nodeTokens = ConditionalTokens{
				QuestionMark: tokens.atPos(n.Condition.Range().End),
				Colon:        tokens.atPos(n.TrueResult.Range().End),
			}
		case *hclsyntax.ForExpr:
			forToken := tokens.atPos(n.OpenRange.End)

			var keyToken, commaToken *Token
			var valueToken Token
			if n.KeyVar != "" {
				key := tokens.atPos(forToken.Range().End)
				comma := tokens.atPos(key.Range().End)
				value := tokens.atPos(comma.Range().End)

				keyToken, commaToken, valueToken = &key, &comma, value
			} else {
				valueToken = tokens.atPos(forToken.Range().End)
			}

			var arrowToken *Token
			if n.KeyExpr != nil {
				arrow := tokens.atPos(n.KeyExpr.Range().End)
				arrowToken = &arrow
			}

			var groupToken *Token
			if n.Group {
				group := tokens.atPos(n.ValExpr.Range().End)
				groupToken = &group
			}

			var ifToken *Token
			if n.CondExpr != nil {
				pos := n.ValExpr.Range().End
				if groupToken != nil {
					pos = groupToken.Range().End
				}
				ift := tokens.atPos(pos)
				ifToken = &ift
			}

			nodeTokens = ForTokens{
				Open:  tokens.atPos(n.OpenRange.Start),
				For:   forToken,
				Key:   keyToken,
				Comma: commaToken,
				Value: valueToken,
				In:    tokens.atPos(valueToken.Range().End),
				Colon: tokens.atPos(n.CollExpr.Range().End),
				Arrow: arrowToken,
				Group: groupToken,
				If:    ifToken,
				Close: tokens.atPos(n.CloseRange.Start),
			}
		case *hclsyntax.FunctionCallExpr:
			nodeTokens = FunctionCallTokens{
				Name:       tokens.atPos(n.NameRange.Start),
				OpenParen:  tokens.atPos(n.OpenParenRange.Start),
				CloseParen: tokens.atPos(n.CloseParenRange.Start),
			}
		case *hclsyntax.IndexExpr:
			nodeTokens = IndexTokens{
				OpenBracket:  tokens.atPos(n.OpenRange.Start),
				CloseBracket: tokens.atOffset(n.BracketRange.End.Byte - 1),
			}
		case *hclsyntax.LiteralValueExpr:
			nodeTokens = LiteralValueTokens{
				Value: tokens.inRange(n.Range()),
			}
		case *hclsyntax.ObjectConsExpr:
			items := make([]ObjectConsItemTokens, len(n.Items))
			for i, item := range n.Items {
				var comma *Token
				if t := tokens.atPos(item.ValueExpr.Range().End); t.Raw.Type == hclsyntax.TokenComma {
					comma = &t
				}
				items[i] = ObjectConsItemTokens{
					Equals: tokens.atPos(item.KeyExpr.Range().End),
					Comma:  comma,
				}
			}
			nodeTokens = ObjectConsTokens{
				OpenBrace:  tokens.atPos(n.OpenRange.Start),
				CloseBrace: tokens.atOffset(n.SrcRange.End.Byte - 1),
			}
		case *hclsyntax.RelativeTraversalExpr:
			nodeTokens = RelativeTraversalTokens{
				Traversal: mapRelativeTraversalTokens(tokens, n.Traversal),
			}
		case *hclsyntax.ScopeTraversalExpr:
			nodeTokens = ScopeTraversalTokens{
				Root:      tokens.atPos(n.Traversal[0].SourceRange().Start),
				Traversal: mapRelativeTraversalTokens(tokens, n.Traversal[1:]),
			}
		case *hclsyntax.SplatExpr:
			openToken := tokens.atOffset(n.MarkerRange.Start.Byte - 1)
			starToken := tokens.atPos(n.MarkerRange.Start)
			var closeToken *Token
			if openToken.Raw.Type == hclsyntax.TokenOBrack {
				cbrack := tokens.atPos(n.MarkerRange.End)
				closeToken = &cbrack
			}
			nodeTokens = SplatTokens{
				Open:  openToken,
				Star:  starToken,
				Close: closeToken,
			}
		case *hclsyntax.TemplateExpr:
			// TODO(pdg): interpolations, control, etc.
			nodeTokens = TemplateTokens{
				Open:  tokens.atPos(n.SrcRange.Start),
				Close: tokens.atOffset(n.SrcRange.End.Byte - 1),
			}
		case *hclsyntax.TupleConsExpr:
			exprs := n.Exprs
			commas := make([]Token, 0, len(exprs))
			for _, ex := range exprs[:len(exprs)-1] {
				commas = append(commas, tokens.atPos(ex.Range().End))
			}
			if trailing := tokens.atPos(exprs[len(exprs)-1].Range().End); trailing.Raw.Type == hclsyntax.TokenComma {
				commas = append(commas, trailing)
			}
			nodeTokens = TupleConsTokens{
				OpenBracket:  tokens.atPos(n.OpenRange.Start),
				Commas:       commas,
				CloseBracket: tokens.atOffset(n.SrcRange.End.Byte - 1),
			}
		case *hclsyntax.UnaryOpExpr:
			nodeTokens = UnaryOpTokens{
				Operator: tokens.atPos(n.SymbolRange.Start),
			}
		}
		if nodeTokens != nil {
			tokenMap[n] = nodeTokens
		}

		return nil
	})
	contract.Assert(diags == nil)

	// If there is a trailing end-of-file token (and there should be), attach it to the top-level body.
	if len(tokens) > 0 && tokens[len(tokens)-1].Raw.Type == hclsyntax.TokenEOF {
		tokenMap[body] = BodyTokens{EndOfFile: tokens[len(tokens)-1]}
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
			items[i] = BracketTraverserTokens{
				OpenBracket:  leadingToken,
				Index:        indexToken,
				CloseBracket: tokens.atOffset(rng.End.Byte - 1),
			}
		} else {
			items[i] = DotTraverserTokens{
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
