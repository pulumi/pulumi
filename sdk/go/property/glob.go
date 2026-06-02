// Copyright 2026, Pulumi Corporation.
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

package property

import (
	"encoding"
	"errors"
	"fmt"
	"iter"
	"math"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Glob struct{ pathRepr }

func GlobFromSegments(segments ...GlobSegment) Glob {
	return Glob{pathReprFromSegments(segments)}
}

// MustParseGlob parses text into a [Glob], panicking on parse errors.
//
// It is intended for tests and other contexts where the input is a known-good literal.
func MustParseGlob(text string) Glob {
	var g Glob
	if err := g.UnmarshalText([]byte(text)); err != nil {
		panic(err)
	}
	return g
}

var (
	_ encoding.TextMarshaler   = Glob{}
	_ encoding.TextUnmarshaler = &Glob{}
)

func (g Glob) MarshalText() (text []byte, err error) {
	if g.len() == 0 {
		return nil, errors.New("cannot marshal an empty glob")
	}
	var b strings.Builder
segment:
	for i, p := range g.enumerate {
		switch p := p.(type) {
		case KeySegment:
			bare := len(p.string) > 0
			for j, c := range p.string {
				if !isPlainPathCharacter(c, j == 0) {
					bare = false
					break
				}
			}
			if !bare {
				fmt.Fprintf(&b, "[%q]", p.string)
				continue segment
			}
			if i != 0 {
				b.WriteRune('.')
			}
			b.WriteString(p.string)
		case IndexSegment:
			b.WriteRune('[')
			b.WriteString(strconv.FormatInt(int64(p.Index()), 10))
			b.WriteRune(']')
		case GlobSegment:
			b.WriteString("[*]")
		default:
			contract.Failf("unknown glob segment %T", p)
		}
	}
	return []byte(b.String()), nil
}

func (g *Glob) UnmarshalText(text []byte) error {
	*g = Glob{}
	if len(text) == 0 {
		return errors.New("cannot unmarshal an empty property path")
	}

	runes := []rune(string(text))
	for len(runes) > 0 {
		switch {
		case runes[0] == '*' && g.len() == 0:
			g.pathRepr = g.appendSplat()
			runes = runes[1:]
		case isPlainPathCharacter(runes[0], true) && g.len() == 0:
			key, remainder, err := parseKey(runes)
			if err != nil {
				return err
			}
			runes = remainder
			g.pathRepr = g.appendGlobSegment(key)
		case runes[0] == '[':
			seg, remainder, err := parseIndex(runes)
			if err != nil {
				return err
			}
			runes = remainder
			g.pathRepr = g.appendGlobSegment(seg)
		case runes[0] == '.':
			if len(runes) > 1 && runes[1] == '*' {
				g.pathRepr = g.appendSplat()
				runes = runes[2:]
				continue
			}
			key, remainder, err := parseKey(runes[1:])
			if err != nil {
				return err
			}
			runes = remainder
			g.pathRepr = g.appendGlobSegment(key)
		default:
			return fmt.Errorf("unknown character '%c' at position %d",
				runes[0], len([]rune(string(text)))-len(runes))
		}
	}

	return nil
}

func isPlainPathCharacter(c rune, first bool) bool {
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
		return true
	}
	return !first && isNumberCharacter(c)
}

func isNumberCharacter(c rune) bool { return c >= '0' && c <= '9' }

func parseKey(runes []rune) (GlobSegment, []rune, error) {
	if len(runes) == 0 {
		return nil, nil, errors.New("expected character")
	}
	var s strings.Builder
	for i := 0; i < len(runes); i++ {
		if !isPlainPathCharacter(runes[i], i == 0) {
			break
		}
		s.WriteRune(runes[i])
	}
	if s.Len() == 0 {
		return nil, runes, fmt.Errorf("expected letter, found '%c'", runes[0])
	}
	runes = runes[s.Len():]

	return KeySegment{s.String()}, runes, nil
}

func parseIndex(runes []rune) (GlobSegment, []rune, error) {
	if len(runes) == 0 || runes[0] != '[' {
		return nil, nil, errors.New("expected '['")
	}
	runes = runes[1:]
	if len(runes) == 0 {
		return nil, nil, errors.New("unclosed '['")
	}

	switch {
	case len(runes) > 1 && runes[0] == '-' && isNumberCharacter(runes[1]):
		return nil, nil, errors.New("indexes cannot be negative")
	case isNumberCharacter(runes[0]):
		i := 1
		for ; ; i++ {
			if len(runes) <= i {
				return nil, nil, fmt.Errorf("unclosed number [%s", string(runes[:i]))
			}
			if !isNumberCharacter(runes[i]) {
				break
			}
		}
		if len(runes) < i || runes[i] != ']' {
			return nil, nil, fmt.Errorf("unclosed index [%s", string(runes[:i]))
		}
		n, err := strconv.ParseUint(string(runes[0:i]), 10, 64)
		if err != nil {
			return nil, nil, err
		}
		if n > math.MaxInt64 {
			return nil, nil, fmt.Errorf("indexes cannot exceed %d", int64(math.MaxInt64))
		}
		return IndexSegment{n}, runes[i+1:], nil
	case runes[0] == '"':
		i := 1
		for ; ; i++ {
			if len(runes) <= i {
				return nil, nil, fmt.Errorf(`unclosed string [%s`, string(runes))
			}
			if runes[i] == '"' {
				if len(runes) <= i+1 || runes[i+1] != ']' {
					return nil, nil, fmt.Errorf(`unclosed index [%s`, string(runes[:i+1]))
				}
				key, err := strconv.Unquote(string(runes[:i+1]))
				return KeySegment{key}, runes[i+2:], err
			}
			if runes[i] == '\\' {
				i++
			}
		}
	case runes[0] == '*':
		if len(runes) == 1 || runes[1] != ']' {
			return nil, nil, errors.New(`expected ']' after "[*"`)
		}
		return Splat, runes[2:], nil
	default:
		return nil, nil, errors.New("unexpected character after '['")
	}
}

func (g Glob) Get(v Value) (map[Path]Value, error) {
	type entry struct {
		path pathRepr
		val  Value
	}
	stack := []entry{{val: v}}
	for segment := range g.segments {
		for i := len(stack) - 1; i >= 0; i-- {
			e := stack[i]
			var expansion []entry
			switch segment := segment.(type) {
			case SplatSegment:
				switch {
				case e.val.IsMap():
					for k, v := range e.val.AsMap().AllStable {
						expansion = append(expansion, entry{
							path: e.path.appendKey(k),
							val:  v,
						})
					}
				case e.val.IsArray():
					for j, v := range e.val.AsArray().AsSlice() {
						expansion = append(expansion, entry{
							path: e.path.appendIndex(uint64(j)), //nolint:gosec // j will always be >= 0
							val:  v,
						})
					}
				default:
					return nil, pathErrorf(e.val, "expected a map or array, found %s", typeString(e.val))
				}
			default:
				vals, err := segment.globApply(e.val)
				if err != nil {
					return nil, err
				}
				for _, v := range vals {
					expansion = append(expansion, entry{
						path: e.path.appendGlobSegment(segment),
						val:  v,
					})
				}
			}
			if len(expansion) == 0 {
				if len(stack) == 1 {
					// We have expanded the last expression out to nothing, so we are done.
					return nil, nil
				}
				stack[i] = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			} else {
				stack[i] = expansion[0]
				stack = append(stack, expansion[1:]...)
			}
		}
	}
	if len(stack) == 0 {
		return nil, nil
	}
	result := make(map[Path]Value, len(stack))
	for _, e := range stack {
		result[Path{pathRepr: e.path}] = e.val
	}
	return result, nil
}

type GlobSegment interface {
	fmt.GoStringer
	globApply(Value) ([]Value, PathApplyFailure)
}

var (
	_ GlobSegment = Splat
	_ GlobSegment = KeySegment{}
	_ GlobSegment = IndexSegment{}
)

var Splat SplatSegment

type SplatSegment struct{}

func (SplatSegment) GoString() string   { return "property.Splat" }
func (s KeySegment) GoString() string   { return fmt.Sprintf("property.NewSegment(%q)", s.string) }
func (i IndexSegment) GoString() string { return fmt.Sprintf("property.NewSegment(%d)", i.i) }

func (SplatSegment) globApply(v Value) ([]Value, PathApplyFailure) {
	switch {
	case v.IsMap():
		values := make([]Value, 0, v.AsMap().Len())
		for _, v := range v.AsMap().AllStable {
			values = append(values, v)
		}
		return values, nil
	case v.IsArray():
		return v.AsArray().AsSlice(), nil
	default:
		return nil, pathErrorf(v, "expected a map or array, found %s", typeString(v))
	}
}

func (s KeySegment) globApply(v Value) ([]Value, PathApplyFailure) {
	r, err := s.apply(v)
	if err != nil {
		return nil, err
	}
	return []Value{r}, nil
}

func (s IndexSegment) globApply(v Value) ([]Value, PathApplyFailure) {
	r, err := s.apply(v)
	if err != nil {
		return nil, err
	}
	return []Value{r}, nil
}

func (s Path) AsGlob() Glob { return Glob{s.pathRepr} }

// Matches returns true if the receiver glob matches the beginning of the given path.
//
// For example, the glob `foo["bar"][1]` matches `foo.bar[1].baz`. The glob segment
// `[*]` is a wildcard which matches any single segment at that nesting level. So for
// example, the glob `foo.*.baz` matches `foo.bar.baz.bam`, and the glob `*` matches
// any path.
func (g Glob) Matches(p Path) bool {
	if p.len() < g.len() {
		return false
	}
	opF, stop := iter.Pull(p.segments)
	defer stop()
	for gp := range g.segments {
		op, ok := opF()
		contract.Assertf(ok, "p.len() should be >= g.len()")
		switch gp := gp.(type) {
		case KeySegment:
			if v, ok := op.(KeySegment); ok && v == gp {
				continue
			}
			return false
		case IndexSegment:
			if v, ok := op.(IndexSegment); ok && v == gp {
				continue
			}
			return false
		case SplatSegment:
			continue
		default:
			contract.Failf("invalid glob segment %T", gp)
		}
	}
	return true
}

func (g Glob) Segments(yield func(GlobSegment) bool) {
	for v := range g.segments {
		if !yield(v) {
			return
		}
	}
}
