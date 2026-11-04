// Copyright 2016, Pulumi Corporation.
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
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

type printable interface {
	print(w io.Writer, p *printer)

	// HasLeadingTrivia returns true if the value has associated leading trivia.
	HasLeadingTrivia() bool
	// HasTrailingTrivia returns true if the value has associated trailing trivia.
	HasTrailingTrivia() bool
	// GetLeadingTrivia returns the leading trivia for this value, if any.
	GetLeadingTrivia() syntax.TriviaList
	// GetTrailingTrivia returns the trailing trivia for this value, if any.
	GetTrailingTrivia() syntax.TriviaList
}

type printer struct {
	indent string

	// itemStart is true while a body item prints and its first token has not printed yet.
	itemStart bool
	// itemPrefix is the text that the printer wrote before the body item on the item's first line.
	itemPrefix string
	// moved holds the comments of the body item that is printing that print after the item's leading trivia and not
	// in its trailing trivia.
	moved []syntax.Comment
}

type formatter func(f fmt.State, c rune)

func (fn formatter) Format(f fmt.State, c rune) {
	fn(f, c)
}

func (p *printer) indented(f func()) {
	p.indent += "    "
	f()
	p.indent = p.indent[:len(p.indent)-4]
}

func (p *printer) format(f fmt.State, c rune, pp printable) {
	if f.Flag(' ') && !pp.HasLeadingTrivia() {
		switch pp.(type) {
		case BodyItem:
			p.fprintf(f, "%s", p.indent)
		case Expression:
			p.fprintf(f, " ")
		}
	}

	parentPrecedence, hasPrecedence := f.Precision()
	if !hasPrecedence {
		pp.print(f, p)
		return
	}

	var operator *hclsyntax.Operation
	switch pp := pp.(type) {
	case *BinaryOpExpression:
		operator = pp.Operation
	case *UnaryOpExpression:
		operator = pp.Operation
	}

	precedence := operatorPrecedence(operator)
	switch {
	case precedence < parentPrecedence || (precedence == parentPrecedence && c == 'o'):
		p.fprintf(f, "(")
		pp.print(f, p)
		p.fprintf(f, ")")
	default:
		pp.print(f, p)
	}
}

func (p *printer) fprintf(w io.Writer, f string, v ...any) {
	for i, e := range v {
		if printable, ok := e.(printable); ok {
			v[i] = formatter(func(f fmt.State, c rune) {
				p.format(f, c, printable)
			})
			continue
		}
		switch e := e.(type) {
		case syntax.Token:
			v[i] = formatter(func(f fmt.State, c rune) {
				p.printToken(f, e)
			})
		case *syntax.Token:
			if e != nil {
				v[i] = formatter(func(f fmt.State, c rune) {
					p.printToken(f, *e)
				})
			}
		case syntax.TriviaList:
			v[i] = formatter(func(f fmt.State, c rune) {
				p.printTrailingTrivia(f, e)
			})
		}
	}

	if _, err := fmt.Fprintf(w, f, v...); err != nil {
		panic(err)
	}
}

// printToken prints a token like syntax.Token.Format, but lays out the generated comments in its trivia.
func (p *printer) printToken(f fmt.State, t syntax.Token) {
	leading, prefix := t.LeadingTrivia, ""
	if p.itemStart {
		p.itemStart, prefix = false, p.itemPrefix
		if len(p.moved) > 0 {
			leading = append(syntax.TriviaList{}, leading...)
			for _, c := range p.moved {
				leading = append(leading, c)
			}
		}
	}

	if leading != nil {
		p.printLeadingTrivia(f, leading, prefix)
	} else if f.Flag(' ') {
		p.write(f, []byte{' '})
	}
	// A token without trivia prints only its text.
	if _, err := fmt.Fprintf(f, "%v", syntax.Token{Raw: t.Raw}); err != nil {
		panic(err)
	}
	p.printTrailingTrivia(f, t.TrailingTrivia)
}

// printLeadingTrivia prints trivia that precedes a token. The trivia starts after prefix on its line.
//
// A generated comment owns the lines that it prints: each line after the first, and the token after the comment,
// starts at the column of the comment's first line. If whitespace trivia follows the comment, that trivia sets the
// position of the token instead.
func (p *printer) printLeadingTrivia(w io.Writer, trivia syntax.TriviaList, prefix string) {
	for i, t := range trivia {
		c, ok := t.(syntax.Comment)
		if !ok || !isGeneratedComment(c) {
			p.write(w, t.Bytes())
			continue
		}

		indent := p.lineIndent(trivia[:i], prefix)
		nextIsWhitespace := false
		if i+1 < len(trivia) {
			_, nextIsWhitespace = trivia[i+1].(syntax.Whitespace)
		}
		p.printComment(w, c, indent, !nextIsWhitespace)
	}
}

// printTrailingTrivia prints trivia that follows a token. It omits the comments that print before the body item.
//
// A generated comment in trailing trivia has nothing to align with, so each line after the first starts at the
// current indentation.
func (p *printer) printTrailingTrivia(w io.Writer, trivia syntax.TriviaList) {
	for _, t := range withoutComments(trivia, p.moved) {
		if c, ok := t.(syntax.Comment); ok && isGeneratedComment(c) {
			p.printComment(w, c, p.indent, false)
		} else {
			p.write(w, t.Bytes())
		}
	}
}

// printComment prints the lines of a generated comment, which ends with a newline. It writes indent before each line
// after the first, and after the final newline if indentLast is true.
func (p *printer) printComment(w io.Writer, c syntax.Comment, indent string, indentLast bool) {
	text := bytes.TrimSuffix(c.Bytes(), []byte{'\n'})
	p.write(w, bytes.ReplaceAll(text, []byte{'\n'}, []byte("\n"+indent)))
	p.write(w, []byte{'\n'})
	if indentLast {
		p.write(w, []byte(indent))
	}
}

// lineIndent returns the whitespace between the start of the current line and the end of the trivia, which starts
// after prefix. If the line holds text that is not whitespace, lineIndent returns the current indentation.
func (p *printer) lineIndent(trivia syntax.TriviaList, prefix string) string {
	var text []byte
	for _, t := range trivia {
		text = append(text, t.Bytes()...)
	}
	line := prefix + string(text)
	if nl := bytes.LastIndexByte(text, '\n'); nl != -1 {
		line = string(text[nl+1:])
	}
	if strings.Trim(line, " \t") != "" {
		return p.indent
	}
	return line
}

func (p *printer) write(w io.Writer, b []byte) {
	if _, err := w.Write(b); err != nil {
		panic(err)
	}
}

// isGeneratedComment returns true if the comment was built from its Lines and not parsed from source. Only a parsed
// comment has a source range.
func isGeneratedComment(c syntax.Comment) bool {
	return c.Range() == (hcl.Range{}) && len(c.Lines) > 0
}

// movedComments returns the generated comments in trivia that span more than one line. A trailing comment of this
// kind cannot follow its token on the same line, so it prints before the body item.
func movedComments(trivia syntax.TriviaList) []syntax.Comment {
	var moved []syntax.Comment
	for _, t := range trivia {
		if c, ok := t.(syntax.Comment); ok && isGeneratedComment(c) && bytes.Count(c.Bytes(), []byte{'\n'}) > 1 {
			moved = append(moved, c)
		}
	}
	return moved
}

// withoutComments returns trivia without the given comments and without the whitespace directly before each of them.
func withoutComments(trivia syntax.TriviaList, comments []syntax.Comment) syntax.TriviaList {
	if len(comments) == 0 {
		return trivia
	}
	result := make(syntax.TriviaList, 0, len(trivia))
	for _, t := range trivia {
		if c, ok := t.(syntax.Comment); ok && containsComment(comments, c) {
			for len(result) > 0 {
				if _, ok := result[len(result)-1].(syntax.Whitespace); !ok {
					break
				}
				result = result[:len(result)-1]
			}
			continue
		}
		result = append(result, t)
	}
	return result
}

// containsComment returns true if comments holds c. Two generated comments are the same comment if they share the
// backing array of their Lines.
func containsComment(comments []syntax.Comment, c syntax.Comment) bool {
	for _, other := range comments {
		if len(c.Lines) > 0 && &c.Lines[0] == &other.Lines[0] {
			return true
		}
	}
	return false
}
