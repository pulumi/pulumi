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
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Glob []GlobSegment

var (
	_ encoding.TextMarshaler   = Glob{}
	_ encoding.TextUnmarshaler = &Glob{}
)

func (g Glob) MarshalText() (text []byte, err error) {
	if len(g) == 0 {
		return nil, errors.New("cannot marshal an empty glob")
	}
	var b strings.Builder
segment:
	for i, p := range g {
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
			b.WriteString(strconv.FormatInt(int64(p.int), 10))
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
	*g = (*g)[:0]
	if len(text) == 0 {
		return errors.New("cannot unmarshal an empty property path")
	}

	runes := []rune(string(text))
	for len(runes) > 0 {
		switch {
		case runes[0] == '*' && len(*g) == 0:
			*g = append(*g, Splat)
			runes = runes[1:]
		case isPlainPathCharacter(runes[0], true) && len(*g) == 0:
			key, remainder, err := parseKey(runes)
			if err != nil {
				return err
			}
			runes = remainder
			(*g) = append((*g), key)
		case runes[0] == '[':
			seg, remainder, err := parseIndex(runes)
			if err != nil {
				return err
			}
			runes = remainder
			(*g) = append((*g), seg)
		case runes[0] == '.':
			if len(runes) > 1 && runes[1] == '*' {
				*g = append(*g, Splat)
				runes = runes[2:]
				continue
			}
			key, remainder, err := parseKey(runes[1:])
			if err != nil {
				return err
			}
			runes = remainder
			(*g) = append((*g), key)

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
		n, err := strconv.Atoi(string(runes[0:i]))
		if err != nil {
			return nil, nil, err
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

func (g Glob) Get(v Value) ([]Value, error) {
	stack := []Value{v}
	for _, segment := range g {
		for i := len(stack) - 1; i >= 0; i-- {
			var err PathApplyFailure
			expansion, err := segment.globApply(stack[i])
			if err != nil {
				return nil, err
			}
			if len(expansion) == 0 {
				if len(stack) == 1 {
					// We have expanded the last expression out to nothing, so we are done.
					return nil, nil
				}
				stack[i] = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			stack[i] = expansion[0]
			stack = append(stack, expansion[1:]...)
		}
	}
	return stack, nil
}

type GlobSegment interface {
	globApply(Value) ([]Value, PathApplyFailure)
}

var (
	_ GlobSegment = Splat
	_ GlobSegment = KeySegment{}
	_ GlobSegment = IndexSegment{}
)

var Splat splat

type splat struct{}

func (splat) globApply(v Value) ([]Value, PathApplyFailure) {
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

func (s Path) AsGlob() Glob {
	g := make(Glob, len(s))
	for i, v := range s {
		g[i] = v
	}
	return g
}

// Matches returns true if the receiver glob matches the beginning of the given path.
//
// For example, the glob `foo["bar"][1]` matches `foo.bar[1].baz`. The glob segment
// `[*]` is a wildcard which matches any single segment at that nesting level. So for
// example, the glob `foo.*.baz` matches `foo.bar.baz.bam`, and the glob `*` matches
// any path.
func (g Glob) Matches(p Path) bool {
	if len(p) < len(g) {
		return false
	}
	for i := range g {
		op := p[i]
		switch gp := g[i].(type) {
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
		case splat:
			continue
		default:
			contract.Failf("invalid glob segment %T", gp)
		}
	}
	return true
}
