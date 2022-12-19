// Copyright 2016-2022, Pulumi Corporation.
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

// pretty is an extensible utility library to pretty-print nested structures.
package pretty

import (
	"fmt"
	"sort"
	"strings"
)

const (
	DefaultColumns int    = 100
	DefaultIndent  string = "  "
)

// A formatter understands how to turn itself into a string while respecting a desired
// column target.
type Formatter interface {
	fmt.Stringer

	// Set the number of columns to print out.
	// This method does not mutate.
	Columns(int) Formatter
}

// Indent a (multi-line) string, passing on column adjustments.
type indent struct {
	// The prefix to be applied to each line
	prefix string
	inner  Formatter
}

func sanitizeColumns(i int) int {
	if i <= 0 {
		return 1
	}
	return i
}

func (i indent) Columns(columns int) Formatter {
	i.inner = i.inner.Columns(columns - len(i.prefix))
	return i
}

func (i indent) String() string {
	lines := strings.Split(i.inner.String(), "\n")
	for j, l := range lines {
		lines[j] = i.prefix + l
	}
	return strings.Join(lines, "\n")
}

// Create a new Formatter of a raw string.
func FromString(s string) Formatter {
	return &literal{s: s}
}

// Create a new Formatter from a fmt.Stringer.
func FromStringer(s fmt.Stringer) Formatter {
	return &literal{t: s}
}

// A string literal that implements Formatter (ignoring Column).
type literal struct {
	// A source for a string.
	t fmt.Stringer
	// A raw string.
	s string
}

func (b *literal) String() string {
	// If we don't have a cached value, but we can compute one,
	if b.s == "" && b.t != nil {
		// Set the known value to the computed value
		b.s = b.t.String()
		// Nil the value source, since we won't need it again, and it might produce "".
		b.t = nil
	}
	return b.s
}

func (b literal) Columns(int) Formatter {
	// We are just calling .String() here, so we can't do anything with columns.
	return &b
}

// A Formatter that wraps an inner value with prefixes and postfixes.
//
// Wrap attempts to respect its column target by changing if the prefix and postfix are on
// the same line as the inner value, or their own lines.
//
// As an example, consider the following instance of Wrap:
//
//	Wrap {
//	  Prefix: "number(", Postfix: ")"
//	  Value: FromString("123456")
//	}
//
// It could be rendered as
//
//	number(123456)
//
// or
//
//	number(
//	  123456
//	)
//
// depending on the column constrains.
type Wrap struct {
	Prefix, Postfix string
	// Require that the Postfix is always on the same line as Value.
	PostfixSameline bool
	Value           Formatter

	columns int
}

func (w Wrap) String() string {
	columns := w.columns
	if columns == 0 {
		columns = DefaultColumns
	}
	inner := w.Value.Columns(columns - len(w.Prefix) - len(w.Postfix)).String()
	lines := strings.Split(inner, "\n")

	if len(lines) == 1 {
		// Full result on one line, and it fits
		if len(inner)+len(w.Prefix)+len(w.Postfix) < columns ||
			// Or its more efficient to include the wrapping instead of the indent.
			len(w.Prefix)+len(w.Postfix) < len(DefaultIndent) {
			return w.Prefix + inner + w.Postfix
		}

		// Print the prefix and postix on their own line, then indent the inner value.
		pre := w.Prefix
		if pre != "" {
			pre += "\n"
		}
		post := w.Postfix
		if post != "" && !w.PostfixSameline {
			post = "\n" + post
		}
		if w.PostfixSameline {
			columns -= len(w.Postfix)
		}
		return pre + indent{
			prefix: DefaultIndent,
			inner:  w.Value,
		}.Columns(columns).String() + post
	}

	// See if we can afford to wrap the prefix & postfix around the first and last lines.
	separate := (w.Prefix != "" && len(w.Prefix)+len(lines[0]) >= columns) ||
		(w.Postfix != "" && len(w.Postfix)+len(lines[len(lines)-1]) >= columns && !w.PostfixSameline)

	if !separate {
		return w.Prefix + strings.TrimSpace(w.Value.Columns(columns).String()) + w.Postfix
	}
	s := w.Prefix
	if w.Prefix != "" {
		s += "\n"
	}
	s += indent{
		prefix: DefaultIndent,
		inner:  w.Value,
	}.Columns(columns).String()
	if w.Postfix != "" && !w.PostfixSameline {
		s += "\n" + w.Postfix
	}
	return s
}

func (w Wrap) Columns(columns int) Formatter {
	w.columns = sanitizeColumns(columns)
	return w
}

// Object is a Formatter that prints string-Formatter pairs, respecting columns where
// possible.
//
// It does this by deciding if the object should be compressed into a single line, or have
// one field per line.
type Object struct {
	Properties map[string]Formatter
	columns    int
}

func (o Object) String() string {
	if len(o.Properties) == 0 {
		return "{}"
	}
	columns := o.columns
	if columns <= 0 {
		columns = DefaultColumns
	}

	// Check if we can do the whole object in a single line
	keys := make([]string, 0, len(o.Properties))
	for key := range o.Properties {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	// Try to build the object in a single line
	singleLine := true
	s := "{ "
	overflowing := func() bool {
		return columns < len(s)-1
	}
	for i, key := range keys {
		s += key + ": "
		v := o.Properties[key].Columns(columns - len(s) - 1).String()
		if strings.IndexRune(v, '\n') != -1 {
			singleLine = false
			break
		}
		if i+1 < len(keys) {
			v += ","
		}
		s += v + " "

		if overflowing() {
			// The object is too big for a single line. Give up and create a multi-line
			// object.
			singleLine = false
			break
		}
	}
	if singleLine {
		return s + "}"
	}

	// reset for a mutl-line object.
	s = "{\n"
	for _, key := range keys {
		s += indent{
			prefix: DefaultIndent,
			inner: Wrap{
				Prefix:          key + ": ",
				Postfix:         ",",
				PostfixSameline: true,
				Value:           o.Properties[key],
			},
		}.Columns(columns).String() + "\n"
	}
	return s + "}"
}

func (o Object) Columns(columns int) Formatter {
	o.columns = sanitizeColumns(columns)
	return o
}

// An ordered set of items displayed with a separator between them.
//
// Items can be displayed on a single line if it fits within the column constraint.
// Otherwise items will be displayed across multiple lines.
type List struct {
	Elements        []Formatter
	Separator       string
	AdjoinSeparator bool

	columns int
}

func (l List) String() string {
	columns := l.columns
	if columns <= 0 {
		columns = DefaultColumns
	}
	s := ""
	singleLine := true
	for i, el := range l.Elements {
		v := el.Columns(columns - len(s)).String()
		if strings.IndexRune(v, '\n') != -1 {
			singleLine = false
			break
		}
		s += v
		if i+1 < len(l.Elements) {
			s += l.Separator
		}

		if len(s) > columns {
			singleLine = false
			break
		}
	}
	if singleLine {
		return s
	}
	s = ""
	if l.AdjoinSeparator {
		separator := strings.TrimRight(l.Separator, " ")
		for i, el := range l.Elements {
			v := el.Columns(columns - len(separator)).String()
			if i+1 != len(l.Elements) {
				v += separator + "\n"
			}
			s += v
		}
		return s
	}

	separator := strings.TrimLeft(l.Separator, " ")
	for i, el := range l.Elements {
		v := indent{
			prefix: strings.Repeat(" ", len(separator)),
			inner:  el,
		}.Columns(columns).String()
		if i != 0 {
			v = "\n" + separator + v[len(separator):]
		}
		s += v
	}
	return s

}

func (l List) Columns(columns int) Formatter {
	l.columns = sanitizeColumns(columns)
	return l
}
