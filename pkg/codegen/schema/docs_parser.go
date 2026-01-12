// Copyright 2020-2024, Pulumi Corporation.
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

package schema

import (
	"bytes"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/pgavlin/goldmark"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/parser"
	"github.com/pgavlin/goldmark/text"
	"github.com/pgavlin/goldmark/util"
)

const (
	// ExamplesShortcode is the name for the `{{% examples %}}` shortcode, which demarcates a set of example sections.
	ExamplesShortcode = "examples"

	// ExampleShortcode is the name for the `{{% example %}}` shortcode, which demarcates the content for a single
	// example.
	ExampleShortcode = "example"

	// RefShortcode is the name for the `{{% ref %}}` shortcode, which references a schema entity.
	RefShortcode = "ref"
)

// Shortcode represents a shortcode element and its contents, e.g. `{{% examples %}}`.
type Shortcode struct {
	ast.BaseBlock

	// Name is the name of the shortcode.
	Name []byte
}

func (s *Shortcode) Dump(w io.Writer, source []byte, level int) {
	m := map[string]string{
		"Name": string(s.Name),
	}
	ast.DumpHelper(w, s, source, level, m, nil)
}

// KindShortcode is an ast.NodeKind for the Shortcode node.
var KindShortcode = ast.NewNodeKind("Shortcode")

// Kind implements ast.Node.Kind.
func (*Shortcode) Kind() ast.NodeKind {
	return KindShortcode
}

// NewShortcode creates a new shortcode with the given name.
func NewShortcode(name []byte) *Shortcode {
	return &Shortcode{Name: name}
}

// Ref represents an inline reference to a schema entity, e.g. `{{% ref #/resources/pkg:index:res %}}`.
type Ref struct {
	ast.BaseInline

	// Destination is the reference destination (e.g. "#/resources/pkg:index:res").
	Destination string
}

func (r *Ref) Dump(w io.Writer, source []byte, level int) {
	m := map[string]string{
		"Destination": r.Destination,
	}
	ast.DumpHelper(w, r, source, level, m, nil)
}

// KindRef is an ast.NodeKind for the Ref node.
var KindRef = ast.NewNodeKind("Ref")

// Kind implements ast.Node.Kind.
func (*Ref) Kind() ast.NodeKind {
	return KindRef
}

// NewRef creates a new Ref with the given destination.
func NewRef(destination string) *Ref {
	return &Ref{Destination: destination}
}

type shortcodeParser int

// NewShortcodeParser returns a BlockParser that parses shortcode (e.g. `{{% examples %}}`).
func NewShortcodeParser() parser.BlockParser {
	return shortcodeParser(0)
}

func (shortcodeParser) Trigger() []byte {
	return []byte{'{'}
}

func (shortcodeParser) parseShortcode(line []byte, pos int) (int, int, int, bool, bool) {
	// Look for `{{%` to open the shortcode.
	text := line[pos:]
	if len(text) < 3 || text[0] != '{' || text[1] != '{' || text[2] != '%' {
		return 0, 0, 0, false, false
	}
	text, pos = text[3:], pos+3

	// Scan through whitespace.
	for {
		if len(text) == 0 {
			return 0, 0, 0, false, false
		}

		r, sz := utf8.DecodeRune(text)
		if !unicode.IsSpace(r) {
			break
		}
		text, pos = text[sz:], pos+sz
	}

	// Check for a '/' to indicate that this is a closing shortcode.
	isClose := false
	if text[0] == '/' {
		isClose = true
		text, pos = text[1:], pos+1
	}

	// Find the end of the name and the closing delimiter (`%}}`) for this shortcode.
	nameStart, nameEnd, inName := pos, pos, true
	for {
		if len(text) == 0 {
			return 0, 0, 0, false, false
		}

		if len(text) >= 3 && text[0] == '%' && text[1] == '}' && text[2] == '}' {
			if inName {
				nameEnd = pos
			}
			pos = pos + 3
			// We don't need to update text
			// because we return after this break.
			break
		}

		r, sz := utf8.DecodeRune(text)
		if inName && unicode.IsSpace(r) {
			nameEnd, inName = pos, false
		}
		text, pos = text[sz:], pos+sz
	}

	return nameStart, nameEnd, pos, isClose, true
}

func (p shortcodeParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 {
		return nil, parser.NoChildren
	}

	nameStart, nameEnd, shortcodeEnd, isClose, ok := p.parseShortcode(line, pos)
	if !ok || isClose {
		return nil, parser.NoChildren
	}
	name := line[nameStart:nameEnd]

	// Skip "ref" shortcodes - they are handled as inline elements.
	if bytes.Equal(name, []byte(RefShortcode)) {
		return nil, parser.NoChildren
	}

	reader.Advance(shortcodeEnd)

	return NewShortcode(name), parser.HasChildren
}

func (p shortcodeParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, seg := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 {
		return parser.Continue | parser.HasChildren
	} else if pos > seg.Len() {
		return parser.Continue | parser.HasChildren
	}

	nameStart, nameEnd, shortcodeEnd, isClose, ok := p.parseShortcode(line, pos)
	if !ok || !isClose {
		return parser.Continue | parser.HasChildren
	}

	shortcode := node.(*Shortcode)
	if !bytes.Equal(line[nameStart:nameEnd], shortcode.Name) {
		return parser.Continue | parser.HasChildren
	}

	reader.Advance(shortcodeEnd)
	return parser.Close
}

func (shortcodeParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
}

// CanInterruptParagraph returns true for shortcodes.
func (shortcodeParser) CanInterruptParagraph() bool {
	return true
}

// CanAcceptIndentedLine returns false for shortcodes; all shortcodes must start at the first column.
func (shortcodeParser) CanAcceptIndentedLine() bool {
	return false
}

type refParser int

// NewRefParser returns an InlineParser that parses ref shortcodes (e.g. `{{% ref #/resources/pkg:index:res %}}`).
func NewRefParser() parser.InlineParser {
	return refParser(0)
}

func (refParser) Trigger() []byte {
	return []byte{'{'}
}

func (refParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 3 || line[0] != '{' || line[1] != '{' || line[2] != '%' {
		return nil
	}

	// Look for the closing `%}}`.
	end := bytes.Index(line, []byte("%}}"))
	if end == -1 {
		return nil
	}

	// Extract content between `{{%` and `%}}`.
	content := line[3:end]

	// Trim leading and trailing whitespace.
	content = bytes.TrimSpace(content)

	// Check if this is a `ref` shortcode.
	if !bytes.HasPrefix(content, []byte("ref")) {
		return nil
	}

	// Extract the destination (everything after "ref").
	destination := bytes.TrimSpace(content[3:])
	if len(destination) == 0 {
		return nil
	}

	// Advance past the entire shortcode.
	block.Advance(end + 3) // advance past `%}}`

	return NewRef(string(destination))
}

// ParseDocs parses the given documentation text as Markdown with shortcodes and returns the AST.
func ParseDocs(docs []byte) ast.Node {
	p := goldmark.DefaultParser()
	p.AddOptions(
		parser.WithBlockParsers(util.Prioritized(shortcodeParser(0), 50)),
		parser.WithInlineParsers(util.Prioritized(NewRefParser(), 50)),
	)
	return p.Parse(text.NewReader(docs))
}
