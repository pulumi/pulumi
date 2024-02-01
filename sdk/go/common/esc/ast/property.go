// Copyright 2023, Pulumi Corporation.
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

package ast

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc/syntax"
)

type PropertyAccess struct {
	Accessors []PropertyAccessor
}

func (p *PropertyAccess) String() string {
	var str strings.Builder
	for _, accessor := range p.Accessors {
		switch accessor := accessor.(type) {
		case *PropertyName:
			if str.Len() != 0 {
				str.WriteByte('.')
			}
			str.WriteString(accessor.Name)
		case *PropertySubscript:
			switch i := accessor.Index.(type) {
			case string:
				fmt.Fprintf(&str, "[\"%s\"]", strings.ReplaceAll(i, `"`, `\"`))
			case int:
				fmt.Fprintf(&str, "[%d]", i)
			}
		}
	}
	return str.String()
}

func (p *PropertyAccess) RootName() string {
	return p.Accessors[0].rootName()
}

type PropertyAccessor interface {
	isAccessor()

	rootName() string

	Range() *hcl.Range
}

type PropertyName struct {
	Name          string
	AccessorRange *hcl.Range
}

func (p *PropertyName) isAccessor() {}

func (p *PropertyName) rootName() string {
	return p.Name
}

func (p *PropertyName) Range() *hcl.Range {
	return p.AccessorRange
}

type PropertySubscript struct {
	Index         interface{}
	AccessorRange *hcl.Range
}

func (p *PropertySubscript) isAccessor() {}

func (p *PropertySubscript) rootName() string {
	return p.Index.(string)
}

func (p *PropertySubscript) Range() *hcl.Range {
	return p.AccessorRange
}

type propertyAccessParser struct {
	parent    syntax.Node
	getRange  func(start, end int) *hcl.Range
	offset    int
	text      string
	accessors []PropertyAccessor
	diags     syntax.Diagnostics
}

func (p *propertyAccessParser) error(start int, msg string) {
	rng := p.rangeFrom(start)
	if rng != nil {
		p.diags.Extend(syntax.Error(rng, msg, p.parent.Syntax().Path()))
	} else {
		p.diags.Extend(syntax.NodeError(p.parent, msg))
	}
}

// Returns a document range starting at start and ending at the current offset.
func (p *propertyAccessParser) rangeFrom(start int) *hcl.Range {
	return p.getRange(start, p.offset)
}

// Returns true if the given byte terminates a property name. Also used during error recovery for
// unterminated numeric subscipts.
func (p *propertyAccessParser) terminatesName(c byte) bool {
	return c == '.' || c == '[' || c == '}' || unicode.IsSpace(rune(c))
}

// Appends a property accessor.
func (p *propertyAccessParser) append(accessor PropertyAccessor) {
	p.accessors = append(p.accessors, accessor)
}

// Consumes a byte of input. Use peek() prior to next() to determine what the byte is if one is
// available. Returns the offset of the consumed  byte.
func (p *propertyAccessParser) next() int {
	at := p.offset
	p.text, p.offset = p.text[1:], p.offset+1
	return at
}

// Returns (but does not consume) the next byte of input, if any.
func (p *propertyAccessParser) peek() (byte, bool) {
	if len(p.text) == 0 {
		return 0, false
	}
	return p.text[0], true
}

// Parses a property access. See the comment on parsePropertyAccess for the grammar and examples.
func (p *propertyAccessParser) parse() (int, string, *PropertyAccess, syntax.Diagnostics) {
	for {
		c, ok := p.peek()
		if !ok {
			p.error(p.offset, "missing closing brace '}' in interpolation")
			return p.finish(p.offset)
		}

		switch c {
		case '}':
			return p.finish(p.next())
		case '.':
			p.append(p.parseName(p.next()))
		case '[':
			p.append(p.parseSubscript(p.next()))
		default:
			if unicode.IsSpace(rune(c)) {
				p.error(p.offset, "missing closing brace '}' in interpolation")
				return p.finish(p.offset)
			}

			start := p.offset
			p.append(p.parseName(start))
		}
	}
}

// Terminates parsing. If there are no accessors (e.g. `${` or `${}`), appends an empty property name
// accessor. Returns the rest of the string, the access, and any diagnostics.
func (p *propertyAccessParser) finish(start int) (int, string, *PropertyAccess, syntax.Diagnostics) {
	if len(p.accessors) == 0 {
		p.append(&PropertyName{Name: "", AccessorRange: p.rangeFrom(start)})
	}

	end := p.offset
	rest := p.text
	access := &PropertyAccess{Accessors: p.accessors}
	return end, rest, access, p.diags
}

// Parses a property name (e.g. `foo`).
func (p *propertyAccessParser) parseName(start int) *PropertyName {
	var b strings.Builder
	for {
		c, ok := p.peek()
		if !ok || p.terminatesName(c) {
			break
		}
		p.next()
		b.WriteByte(c)
	}
	if b.Len() == 0 {
		p.error(start, "property name must not be empty")
	}
	return &PropertyName{Name: b.String(), AccessorRange: p.rangeFrom(start)}
}

// Parses a subscript accessor (e.g. `["foo"]` or `[1]`).
//
// At this point we are already past the opening `[`. Consumes the terminating `]`, if any.
func (p *propertyAccessParser) parseSubscript(start int) *PropertySubscript {
	c, ok := p.peek()
	if !ok {
		p.error(start, "subscript is missing closing bracket ']'")
		return &PropertySubscript{Index: "", AccessorRange: p.rangeFrom(start)}
	}

	var index any
	if c == '"' {
		p.next()
		index = p.parseStringSubscript(start)
	} else {
		index = p.parseIndexSubscript(start)
	}

	c, ok = p.peek()
	if !ok || c != ']' {
		p.error(start, "subscript is missing closing bracket ']'")
	} else {
		p.next()
	}
	return &PropertySubscript{Index: index, AccessorRange: p.rangeFrom(start)}
}

// Parses a string subscript.
//
// At this point we are already past the opening `["`. Ends on EOF or an unescaped `"`. Consumes
// the terminating `"` if any.
func (p *propertyAccessParser) parseStringSubscript(start int) string {
	var propertyKey strings.Builder
	for {
		c, ok := p.peek()
		if !ok {
			// skip the opening brace when reporting the error.
			p.error(start+1, "key subscript is missing closing quote '\"'")
			return propertyKey.String()
		}
		p.next()

		if c == '"' {
			if propertyKey.Len() == 0 {
				// skip the opening brace when reporting the error.
				p.error(start+1, "key subscript must not be empty")
			}
			return propertyKey.String()
		}

		if c == '\\' {
			if n, ok := p.peek(); ok && n == '"' {
				p.next()
				c = n
			}
		}
		propertyKey.WriteByte(c)
	}
}

// Parses an index subscript.
//
// At this point we are already past the opening `[`. Ends on EOF, `]`, or a name terminator.
// Does not consume the terminator.
func (p *propertyAccessParser) parseIndexSubscript(start int) any {
	var index strings.Builder
	for {
		c, ok := p.peek()
		if !ok || c == ']' || p.terminatesName(c) {
			break
		}

		p.next()
		index.WriteByte(c)
	}

	indexStr := index.String()
	num, err := strconv.ParseInt(indexStr, 10, 0)
	if err != nil {
		// skip the opening brace when reporting the error.
		p.error(start+1, "numeric subscript must be a positive base-10 integer")
		return indexStr
	}

	if len(p.accessors) == 0 {
		p.error(start, "the first accessor must be a property name or key subscript, not a numeric subscript")
	}
	return int(num)
}

// parsePropertyAccess parses a property access into a PropertyAccess value.
//
// A property access string is essentially a Javascript property access expression in which all elements are literals.
// Valid property accesses obey the following EBNF-ish grammar:
//
//	propertyName := [a-zA-Z_$] { [a-zA-Z0-9_$] }
//	quotedPropertyName := '"' ( '\' '"' | [^"] ) { ( '\' '"' | [^"] ) } '"'
//	arrayIndex := { [0-9] }
//
//	propertyIndex := '[' ( quotedPropertyName | arrayIndex ) ']'
//	rootProperty := ( propertyName | propertyIndex )
//	propertyAccessor := ( ( '.' propertyName ) |  propertyIndex )
//	path := rootProperty { propertyAccessor }
//
// Examples of valid paths:
// - root
// - root.nested
// - root["nested"]
// - root.double.nest
// - root["double"].nest
// - root["double"]["nest"]
// - root.array[0]
// - root.array[100]
// - root.array[0].nested
// - root.array[0][1].nested
// - root.nested.array[0].double[1]
// - root["key with \"escaped\" quotes"]
// - root["key with a ."]
// - ["root key with \"escaped\" quotes"].nested
// - ["root key with a ."][100]
func parsePropertyAccess(node syntax.Node, start int, access string) (int, string, *PropertyAccess, syntax.Diagnostics) {

	// TODO: diagnostic ranges

	getRange := func(start, end int) *hcl.Range { return nil }
	if scalar, ok := node.Syntax().(syntax.Scalar); ok {
		getRange = scalar.ScalarRange
	}

	p := &propertyAccessParser{
		parent:   node,
		getRange: getRange,
		offset:   start,
		text:     access,
	}
	return p.parse()
}
