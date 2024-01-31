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
}

type PropertyName struct {
	Name string
}

func (p *PropertyName) isAccessor() {}

func (p *PropertyName) rootName() string {
	return p.Name
}

type PropertySubscript struct {
	Index interface{}
}

func (p *PropertySubscript) isAccessor() {}

func (p *PropertySubscript) rootName() string {
	return p.Index.(string)
}

func terminatesName(c byte) bool {
	return c == '.' || c == '[' || c == '}' || unicode.IsSpace(rune(c))
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
func parsePropertyAccess(node syntax.Node, access string) (string, *PropertyAccess, syntax.Diagnostics) {
	// TODO: diagnostic ranges

	var diags syntax.Diagnostics

	// We interpret the grammar above a little loosely in order to keep things simple. Specifically, we will accept
	// something close to the following:
	// pathElement := { '.' } ( '[' ( [0-9]+ | '"' ('\' '"' | [^"] )+ '"' ']' | [a-zA-Z_$][a-zA-Z0-9_$] )
	// path := { pathElement }
	var accessors []PropertyAccessor
outer:
	for len(access) > 0 {
		switch access[0] {
		case '}':
			// interpolation terminator

			// Handle the case of an empty, terminated access (`${}`)
			if len(accessors) == 0 {
				accessors = []PropertyAccessor{&PropertyName{Name: ""}}
			}
			return access[1:], &PropertyAccess{Accessors: accessors}, diags
		case '.':
			if len(accessors) == 0 {
				diags.Extend(syntax.NodeError(node, "the root property must be a string subscript or a name"))
			}
			access = access[1:]
		case '[':
			// If the character following the '[' is a '"', parse a string key.
			if len(access) == 1 {
				access = access[1:]
				break outer
			}
			if access[1] == '"' {
				var propertyKey []byte
				var i int
				for i = 2; ; {
					if i >= len(access) {
						diags.Extend(syntax.NodeError(node, "missing closing quote in property name"))
						i = len(access)
						break
					} else if access[i] == '"' {
						i++
						break
					} else if access[i] == '\\' && i+1 < len(access) && access[i+1] == '"' {
						propertyKey = append(propertyKey, '"')
						i += 2
					} else {
						propertyKey = append(propertyKey, access[i])
						i++
					}
				}
				if i != len(access) {
					if access[i] == ']' {
						i++
					} else {
						diags.Extend(syntax.NodeError(node, "missing closing bracket in property access"))
					}
				}
				accessors, access = append(accessors, &PropertySubscript{Index: string(propertyKey)}), access[i:]
			} else {
				// Look for a closing ']'
				rbracket := strings.IndexRune(access, ']')
				if rbracket == -1 {
					diags.Extend(syntax.NodeError(node, "missing closing bracket in list index"))

					// Look for an alternative terminator
				search:
					for i := 1; ; i++ {
						if i == len(access) || terminatesName(access[i]) {
							rbracket = i
							break search
						}
					}
				}
				if rbracket != 1 {
					index, err := strconv.ParseInt(access[1:rbracket], 10, 0)
					if err != nil {
						diags.Extend(syntax.NodeError(node, "invalid list index"))
					}

					if len(accessors) == 0 {
						diags.Extend(syntax.NodeError(node, "the root property must be a string subscript or a name"))
					}

					rbracket += 1
					accessors = append(accessors, &PropertySubscript{Index: int(index)})
				}
				access = access[rbracket:]
			}
		default:
			if unicode.IsSpace(rune(access[0])) {
				break outer
			}

			for i := 0; ; i++ {
				if i == len(access) || access[i] == '.' || access[i] == '[' || access[i] == '}' || unicode.IsSpace(rune(access[i])) {
					propertyName := access[:i]
					// Ensure the root property is not an integer
					if len(accessors) == 0 {
						if _, err := strconv.ParseInt(propertyName, 10, 0); err == nil {
							diags.Extend(syntax.NodeError(node, "the root property must be a string subscript or a name"))
						}
					}
					accessors, access = append(accessors, &PropertyName{Name: propertyName}), access[i:]
					break
				}
			}
		}
	}
	// Handle the case of an empty, unterminated access (`${`)
	if len(accessors) == 0 {
		accessors = []PropertyAccessor{&PropertyName{Name: ""}}
	}
	diags.Extend(syntax.NodeError(node, "unterminated interpolation"))
	return access, &PropertyAccess{Accessors: accessors}, diags
}
