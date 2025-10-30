// Copyright 2016-2021, Pulumi Corporation.
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

// Helper code to assist emitting correctly minimally parenthesized
// TypeScript type literals.
package tstypes

import (
	"bytes"
)

// Supported types include type identifiers, arrays `T[]`, unions
// `A|B`, and maps with string keys.
type TypeAst interface {
	depth() int
}

// Produces a TypeScript type literal for the type, with minimally
// inserted parentheses.
func TypeLiteral(ast TypeAst) string {
	tokens := (&typeScriptTypeUnparser{}).unparse(ast)
	return toLiteral(tokens)
}

// Builds a type identifier (possibly qualified such as
// "my.module.MyType") or a primitive such as "boolean".
func Identifier(id string) TypeAst {
	return &idType{id}
}

// Builds a `T[]` type from a `T` type.
func Array(t TypeAst) TypeAst {
	return &arrayType{t}
}

// Builds a `{[key: string]: T}` type from a `T` type.
func StringMap(t TypeAst) TypeAst {
	return &mapType{t}
}

// Builds a union `A | B | C` type.
func Union(t ...TypeAst) TypeAst {
	if len(t) == 0 {
		panic("At least one type is needed to form a Union, none are given")
	}
	if len(t) == 1 {
		return t[0]
	}
	return &unionType{t[0], t[1], t[2:]}
}

// Normalizes by unnesting unions `A | (B | C) => A | B | C`.
func Normalize(ast TypeAst) TypeAst {
	return transform(ast, func(t TypeAst) TypeAst {
		switch v := t.(type) {
		case *unionType:
			var all []TypeAst
			for _, e := range v.all() {
				switch ev := e.(type) {
				case *unionType:
					all = append(all, ev.all()...)
				default:
					all = append(all, ev)
				}
			}
			return Union(all...)
		default:
			return t
		}
	})
}

func transform(t TypeAst, f func(x TypeAst) TypeAst) TypeAst {
	switch v := t.(type) {
	case *unionType:
		var ts []TypeAst
		for _, x := range v.all() {
			ts = append(ts, transform(x, f))
		}
		return f(Union(ts...))
	case *arrayType:
		return f(&arrayType{transform(v.arrayElement, f)})
	case *mapType:
		return f(&mapType{transform(v.mapElement, f)})
	default:
		return f(t)
	}
}

type idType struct {
	id string
}

func (*idType) depth() int {
	return 1
}

var _ TypeAst = &idType{}

type mapType struct {
	mapElement TypeAst
}

func (t *mapType) depth() int {
	return t.mapElement.depth() + 1
}

var _ TypeAst = &mapType{}

type arrayType struct {
	arrayElement TypeAst
}

func (t *arrayType) depth() int {
	return t.arrayElement.depth() + 1
}

var _ TypeAst = &arrayType{}

type unionType struct {
	t1    TypeAst
	t2    TypeAst
	tRest []TypeAst
}

func (t *unionType) all() []TypeAst {
	return append([]TypeAst{t.t1, t.t2}, t.tRest...)
}

func (t *unionType) depth() int {
	maxDepth := 0
	for _, t := range t.all() {
		d := t.depth()
		if d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}

var _ TypeAst = &unionType{}

type typeTokenKind string

const (
	openParen  typeTokenKind = "("
	closeParen typeTokenKind = ")"
	openMap    typeTokenKind = "{[key: string]: "
	closeMap   typeTokenKind = "}"
	identifier typeTokenKind = "x"
	array      typeTokenKind = "[]"
	union      typeTokenKind = " | "
)

type typeToken struct {
	kind  typeTokenKind
	value string
}

type typeScriptTypeUnparser struct{}

func (u typeScriptTypeUnparser) unparse(ast TypeAst) []typeToken {
	switch v := ast.(type) {
	case *idType:
		return []typeToken{{identifier, v.id}}
	case *arrayType:
		return append(u.unparseWithUnionParens(v.arrayElement), typeToken{array, ""})
	case *mapType:
		return append([]typeToken{{openMap, ""}}, append(u.unparse(v.mapElement), typeToken{closeMap, ""})...)
	case *unionType:
		var tokens []typeToken
		for i, t := range v.all() {
			if i > 0 {
				tokens = append(tokens, typeToken{union, ""})
			}
			tokens = append(tokens, u.unparseWithUnionParens(t)...)
		}
		return tokens
	default:
		panic("Unknown object of type typeAst")
	}
}

func (u typeScriptTypeUnparser) unparseWithUnionParens(ast TypeAst) []typeToken {
	var parens bool
	switch ast.(type) {
	case *unionType:
		parens = true
	}
	tokens := u.unparse(ast)
	if parens {
		return u.parenthesize(tokens)
	}
	return tokens
}

func (u typeScriptTypeUnparser) parenthesize(tokens []typeToken) []typeToken {
	return append([]typeToken{{openParen, ""}}, append(tokens, typeToken{closeParen, ""})...)
}

func toLiteral(tokens []typeToken) string {
	var buffer bytes.Buffer
	for _, t := range tokens {
		if t.value != "" {
			buffer.WriteString(t.value)
		} else {
			buffer.WriteString(string(t.kind))
		}
	}
	return buffer.String()
}
