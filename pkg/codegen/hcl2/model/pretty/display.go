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
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

const (
	DefaultColumns int    = 100
	DefaultIndent  string = "  "
)

// Run the String() function for a formatter.
func fmtString(f Formatter) string {
	tg := newTagGenerator()
	f.visit(tg.visit)
	return f.string(tg)
}

// The value type for `tagGenerator`.
type visitedFormatter struct {
	// The tag associated with a `Formatter`. If no tag is associated with the
	// `Formatter`, the tag is "".
	tag string
	// The number of occurrences of the `Formatter` at the current node. If the number
	// ever exceeds 1, it indicates the type is recursive.
	count int
}

type tagGenerator struct {
	// Go `Formatter`s already seen in the traversal.
	//
	// valueSeen exists to prevent infinite recursion when visiting types to detect
	// structural recursion. Since all `Formatter`s are pointer types, `valueSeen` allows
	// multiple types that are structurally the same, but different values in memory.
	valueSeen map[Formatter]bool
	// A cache of `Formatter` to their hash values.
	//
	// Since hashing is O(n), it is helpful to store
	knownHashes map[Formatter]string
	// The "hash" of `Formatter`s already seen.
	//
	// structuralSeen exists to prevent infinite recursion when printing types, and
	// operates at the level of structural equality (ignoring pointers).
	structuralSeen map[string]visitedFormatter
	// Type tags are labeled by occurrence, so we keep track of how many have been
	// generated.
	generatedTags int
}

func newTagGenerator() *tagGenerator {
	return &tagGenerator{
		valueSeen:      map[Formatter]bool{},
		knownHashes:    map[Formatter]string{},
		structuralSeen: map[string]visitedFormatter{},
	}
}

// Visit a type.
//
// A function is returned to be called when leaving the type. nil indicates that the type
// is already visited.
func (s *tagGenerator) visit(f Formatter) func() {
	h := s.hash(f)
	seen := s.structuralSeen[h]
	seen.count++
	s.structuralSeen[h] = seen
	if seen.count > 1 {
		return nil
	}
	return func() { c := s.structuralSeen[h]; c.count--; s.structuralSeen[h] = c }
}

func (s *tagGenerator) hash(f Formatter) string {
	h, ok := s.knownHashes[f]
	if !ok {
		h = f.hash(s.valueSeen)
		s.knownHashes[f] = h
	}
	return h
}

// Fetch a tag for a Formatter, if applicable.
//
// If no tag is necessary "", false is returned.
//
// If f is the defining instance of the type and should be labeled with the tag T; T,
// false is returned.
//
// If f is an inner usage of a Formatter with tag T and thus should not be printed; T,
// true is returned.
func (s *tagGenerator) tag(f Formatter) (tag string, tagOnly bool) {
	h := s.hash(f)
	seen, hasValue := s.structuralSeen[h]
	if !hasValue {
		// All values should have been visited before printing.
		panic(fmt.Sprintf("Unexpected new value: h=%q", h))
	}

	// We don't need to tag this type, since it only shows up once
	if seen.count == 0 {
		return "", false
	}

	if seen.tag != "" {
		// We have seen this type before, so we want to return the tag and the tag alone.
		return seen.tag, true
	}

	// We are generating a new tag for a type that needs a tag.
	s.generatedTags++
	tag = fmt.Sprintf("'T%d", s.generatedTags)
	s.structuralSeen[h] = visitedFormatter{
		tag:   tag,
		count: seen.count,
	}
	return tag, false
}

// A formatter understands how to turn itself into a string while respecting a desired
// column target.
type Formatter interface {
	fmt.Stringer

	// Set the number of columns to print out.
	// This method does not mutate.
	Columns(int) Formatter

	// Set the columns for the Formatter and return the receiver.
	columns(int) Formatter
	// An inner print function
	string(tg *tagGenerator) string
	// Visit each underlying Formatter
	visit(visitor func(Formatter) func())
	// A structural id/hash of the underlying Formatter
	hash(seen map[Formatter]bool) string
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

func (i *indent) hash(seen map[Formatter]bool) string {
	seen[i] = true
	return fmt.Sprintf("i%s%s", i.prefix, i.inner.hash(seen))
}

func (i *indent) columns(columns int) Formatter {
	i.inner = i.inner.columns(columns - len(i.prefix))
	return i
}

func (i indent) Columns(columns int) Formatter {
	return i.columns(columns)
}

func (i indent) String() string {
	return fmtString(&i)
}

func (i *indent) visit(visitor func(Formatter) func()) {
	leave := visitor(i)
	if leave == nil {
		return
	}
	defer leave()
	i.inner.visit(visitor)
}

func (i indent) string(tg *tagGenerator) string {
	lines := strings.Split(i.inner.string(tg), "\n")
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

func (b *literal) visit(visiter func(Formatter) func()) {
	// We don't need to do anything here
	leave := visiter(b)
	if leave != nil {
		leave()
	}
}

func (b *literal) string(tg *tagGenerator) string {
	// If we don't have a cached value, but we can compute one,
	if b.s == "" && b.t != nil {
		// Set the known value to the computed value
		b.s = b.t.String()
		// Nil the value source, since we won't need it again, and it might produce "".
		b.t = nil
	}
	return b.s
}

func (b *literal) String() string {
	return fmtString(b)
}

func (b *literal) hash(seen map[Formatter]bool) string {
	seen[b] = true
	return strconv.Quote(b.string(nil))
}

func (b *literal) columns(int) Formatter {
	// We are just calling .String() here, so we can't do anything with columns.
	return b
}

func (b literal) Columns(columns int) Formatter {
	return b.columns(columns)
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

	cols int
}

func (w *Wrap) String() string {
	return fmtString(w)
}

func (w *Wrap) visit(visitor func(Formatter) func()) {
	leave := visitor(w)
	if leave == nil {
		return
	}
	defer leave()
	w.Value.visit(visitor)
}

func (w Wrap) hash(seen map[Formatter]bool) string {
	return fmt.Sprintf("w(%s,%s,%s)", w.Prefix, w.Value.hash(seen), w.Postfix)
}

func (w *Wrap) string(tg *tagGenerator) string {
	columns := w.cols
	if columns == 0 {
		columns = DefaultColumns
	}
	inner := w.Value.columns(columns - len(w.Prefix) - len(w.Postfix)).string(tg)
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
		return pre + (&indent{
			prefix: DefaultIndent,
			inner:  w.Value,
		}).columns(columns).string(tg) + post
	}

	// See if we can afford to wrap the prefix & postfix around the first and last lines.
	separate := (w.Prefix != "" && len(w.Prefix)+len(lines[0]) >= columns) ||
		(w.Postfix != "" && len(w.Postfix)+len(lines[len(lines)-1]) >= columns && !w.PostfixSameline)

	if !separate {
		return w.Prefix + inner + w.Postfix
	}
	s := w.Prefix
	if w.Prefix != "" {
		s += "\n"
	}
	s += (&indent{
		prefix: DefaultIndent,
		inner:  w.Value,
	}).columns(columns).string(tg)
	if w.Postfix != "" && !w.PostfixSameline {
		s += "\n" + w.Postfix
	}
	return s
}

func (w *Wrap) columns(i int) Formatter {
	w.cols = sanitizeColumns(i)
	return w
}

func (w Wrap) Columns(columns int) Formatter {
	return w.columns(columns)
}

// Object is a Formatter that prints string-Formatter pairs, respecting columns where
// possible.
//
// It does this by deciding if the object should be compressed into a single line, or have
// one field per line.
type Object struct {
	Properties map[string]Formatter
	cols       int
}

func (o *Object) String() string {
	return fmtString(o)
}

func (o *Object) hash(seen map[Formatter]bool) string {
	if seen[o] {
		return strconv.Itoa(len(seen))
	}
	defer func() { seen[o] = false }()
	seen[o] = true
	s := "o("
	keys := slice.Prealloc[string](len(o.Properties))
	for key := range o.Properties {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	for i, k := range keys {
		if i != 0 {
			s += ","
		}
		s += k + ":" + o.Properties[k].hash(seen)
	}
	return s + ")"
}

func (o *Object) visit(visiter func(Formatter) func()) {
	leave := visiter(o)
	if leave == nil {
		return
	}
	defer leave()
	// Check if we can do the whole object in a single line
	keys := slice.Prealloc[string](len(o.Properties))
	for key := range o.Properties {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	for _, k := range keys {
		o.Properties[k].visit(visiter)
	}
}

func (o *Object) string(tg *tagGenerator) string {
	if len(o.Properties) == 0 {
		return "{}"
	}
	columns := o.cols
	if columns <= 0 {
		columns = DefaultColumns
	}

	tag, tagOnly := tg.tag(o)
	if tagOnly {
		return tag
	}
	if tag != "" {
		tag += " "
	}

	// Check if we can do the whole object in a single line
	keys := slice.Prealloc[string](len(o.Properties))
	for key := range o.Properties {
		keys = append(keys, key)
	}
	sort.StringSlice(keys).Sort()

	// Try to build the object in a single line
	singleLine := true
	s := tag + "{ "
	overflowing := func() bool {
		return columns < len(s)-1
	}
	for i, key := range keys {
		s += key + ": "
		v := o.Properties[key].columns(columns - len(s) - 1).string(tg)
		if strings.ContainsRune(v, '\n') {
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
	s = tag + "{\n"
	for _, key := range keys {
		s += (&indent{
			prefix: DefaultIndent,
			inner: &Wrap{
				Prefix:          key + ": ",
				Postfix:         ",",
				PostfixSameline: true,
				Value:           o.Properties[key],
			},
		}).columns(columns).string(tg) + "\n"
	}
	return s + "}"
}

func (o *Object) columns(i int) Formatter {
	o.cols = sanitizeColumns(i)
	return o
}

func (o *Object) Columns(columns int) Formatter {
	return o.columns(columns)
}

// An ordered set of items displayed with a separator between them.
//
// Items can be displayed on a single line if it fits within the column constraint.
// Otherwise items will be displayed across multiple lines.
type List struct {
	Elements        []Formatter
	Separator       string
	AdjoinSeparator bool

	cols int
}

func (l *List) String() string {
	return fmtString(l)
}

func (l *List) hash(seen map[Formatter]bool) string {
	if seen[l] {
		return strconv.Itoa(len(seen))
	}
	defer func() { seen[l] = false }()
	seen[l] = true
	s := "(l," + l.Separator
	for _, el := range l.Elements {
		s += "," + el.hash(seen)
	}
	return s + ")"
}

func (l *List) visit(visiter func(Formatter) func()) {
	leave := visiter(l)
	if leave == nil {
		return
	}
	defer leave()
	for _, el := range l.Elements {
		el.visit(visiter)
	}
}

func (l *List) string(tg *tagGenerator) string {
	tag, tagOnly := tg.tag(l)
	if tagOnly {
		return tag
	}
	if tag != "" {
		tag += " "
	}
	columns := l.cols
	if columns <= 0 {
		columns = DefaultColumns
	}
	s := tag
	singleLine := true
	for i, el := range l.Elements {
		v := el.columns(columns - len(s)).string(tg)
		if strings.ContainsRune(v, '\n') {
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
	s = tag
	if l.AdjoinSeparator {
		separator := strings.TrimRight(l.Separator, " ")
		for i, el := range l.Elements {
			v := el.columns(columns - len(separator)).string(tg)
			if i+1 != len(l.Elements) {
				v += separator + "\n"
			}
			s += v
		}
		return s
	}

	separator := strings.TrimLeft(l.Separator, " ")
	for i, el := range l.Elements {
		v := (&indent{
			prefix: strings.Repeat(" ", len(separator)),
			inner:  el,
		}).columns(columns).string(tg)
		if i != 0 {
			v = "\n" + separator + v[len(separator):]
		}
		s += v
	}
	return s
}

func (l *List) columns(i int) Formatter {
	l.cols = sanitizeColumns(i)
	return l
}

func (l List) Columns(columns int) Formatter {
	return l.columns(columns)
}
