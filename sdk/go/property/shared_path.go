// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func pathParse(path []rune) (elements []any, isGlob bool, _ error) {
	if len(path) > 0 && path[0] == '.' {
		return nil, false, errors.New("expected property path to start with a name or index")
	}

	for len(path) > 0 {
		switch path[0] {
		case '.':
			path = path[1:]
			if len(path) == 0 {
				return nil, false, errors.New("expected property path to end with a name or index")
			}
			if path[0] == '[' {
				return nil, false, errors.New("expected property name after '.'")
			}
		case '[':
			path = path[1:]
			if len(path) == 0 {
				return nil, false, errors.New("incomplete index")
			}
			if path[0] == '"' {
				path = path[1:]
				var element strings.Builder
				i := 0
				for {
					if i >= len(path) {
						return nil, false, errors.New("missing closing quote in property name")
					} else if path[i] == '"' {
						// We have found the closing element, finish the element and break
						elements = append(elements, element.String())
						path = path[i+1:]
						break
					} else if path[i] == '\\' {
						// An invalid escape:
						//
						// We will return missing closing bracket
						if i+1 >= len(path) {
							break
						}
						switch path[i+1] {
						case '"':
							element.WriteRune(path[i+1])
						default:
							return nil, false,
								fmt.Errorf("unknown escape sequence: \\%c", path[i+1])
						}
						i += 2
					} else {
						element.WriteRune(path[i])
						i++
					}
				}

				if len(path) == 0 || path[0] != ']' {
					return nil, false, errors.New("missing closing bracket in property access")
				}
				path = path[1:]
			} else {
				idx := slices.Index(path, ']')
				if idx == -1 {
					return nil, false, errors.New("missing closing bracket in index")
				} else if idx == 0 {
					return nil, false, errors.New("missing index value")
				}

				segment := string(path[:idx])
				path = path[idx+1:]

				if segment == "*" {
					isGlob = true
					elements = append(elements, glob)
				} else {
					index, err := strconv.ParseInt(segment, 10, 0)
					if err != nil {
						return nil, false, fmt.Errorf("invalid array index: %w", err)
					}
					elements = append(elements, int(index))
				}
			}

		default: // A index path (.value)
			i := slices.IndexFunc(path, func(c rune) bool {
				return c == '.' || c == '['
			})
			if i == -1 {
				i = len(path)
			}
			elements = append(elements, string(path[:i]))
			path = path[i:]
		}
	}

	return elements, isGlob, nil
}

func pathString(p []any) string {
	var b strings.Builder
	for i, e := range p {
		switch e := e.(type) {
		case int:
			b.WriteRune('[')
			b.WriteString(strconv.Itoa(e))
			b.WriteRune(']')
		case string:
			if strings.ContainsAny(e, ".[]\"") {
				b.WriteString(fmt.Sprintf("[%#v]", e))
			} else {
				if i != 0 {
					b.WriteRune('.')
				}

				b.WriteString(e)
			}
		case Glob:
			b.WriteString("[*]")
		default:
			panic(fmt.Sprintf("Invalid path element of type %T", e))
		}
	}
	return b.String()

}

func pathGoString(starter string, p []any) string {
	var b strings.Builder
	b.WriteString(starter)
	for _, e := range p {
		switch e := e.(type) {
		case int:
			b.WriteString(fmt.Sprintf(".Index(%#v)", e))
		case string:
			b.WriteString(fmt.Sprintf(".Field(%#v)", e))
		case Glob:
			b.WriteString(".Glob()")
		default:
			panic(fmt.Sprintf("Invalid path element of type %T", e))
		}
	}
	return b.String()
}
