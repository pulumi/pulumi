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

package tstypes

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestParenInsert(t *testing.T) {
	t.Parallel()

	ast := astGenerator()
	rapid.Check(t, func(t *rapid.T) {
		example := ast.Draw(t, "ast").(TypeAst)

		t.Logf("example: %s", spew.Sdump(example))
		t.Logf("example type: %s", TypeLiteral(example))

		tokens := (&typeScriptTypeUnparser{}).unparse(example)

		parsed, err := (&typeScriptTypeParser{}).parse(tokens)

		if err != nil {
			t.Error(err)
			return
		}
		t.Logf("parsed: %s", spew.Sdump(parsed))
		assert.Equal(t, example, parsed)
	})
}

func astGenerator() *rapid.Generator {
	names := rapid.OneOf(rapid.Just("x"), rapid.Just("y"))

	var ast func(depth int) *rapid.Generator
	ast = func(depth int) *rapid.Generator {
		if depth <= 1 {
			return rapid.Custom(func(t *rapid.T) TypeAst {
				n := names.Draw(t, "name").(string)
				return &idType{n}
			})
		}

		sub := ast(depth - 1)
		subs := rapid.SliceOfN(sub, 2, 4)

		mapGen := rapid.Custom(func(t *rapid.T) TypeAst {
			element := sub.Draw(t, "ast").(TypeAst)
			return &mapType{element}
		})

		arrGen := rapid.Custom(func(t *rapid.T) TypeAst {
			element := sub.Draw(t, "ast").(TypeAst)
			return &arrayType{element}
		})

		unionGen := rapid.Custom(func(t *rapid.T) TypeAst {
			ts := subs.Draw(t, "asts").([]TypeAst)
			return &unionType{ts[0], ts[1], ts[2:]}
		})

		return rapid.OneOf(mapGen, arrGen, unionGen)
	}

	n := rapid.IntRange(1, 3)
	return rapid.Custom(func(t *rapid.T) TypeAst {
		sz := n.Draw(t, "n").(int)
		return ast(sz).Draw(t, "ast").(TypeAst)
	})
}

type typeScriptTypeParser struct{}

func (p *typeScriptTypeParser) parse(tokens []typeToken) (TypeAst, error) {
	e, rest, err := p.parseType(tokens)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, fmt.Errorf("Unexpected trailing tokens")
	}
	return e, nil
}

func (p *typeScriptTypeParser) parseType(tokens []typeToken) (TypeAst, []typeToken, error) {
	return p.parseType3(tokens)
}

func (p *typeScriptTypeParser) parseType1(tokens []typeToken) (TypeAst, []typeToken, error) {
	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("Expect more tokens")
	}

	switch tokens[0].kind {
	case openParen:
		t, rest, err := p.parseType(tokens[1:])
		if err != nil {
			return nil, nil, err
		}
		if len(rest) == 0 || rest[0].kind != closeParen {
			return nil, nil, fmt.Errorf("Expect `)`")
		}
		return t, rest[1:], nil
	case openMap:
		t, rest, err := p.parseType(tokens[1:])
		if err != nil {
			return nil, nil, err
		}
		if len(rest) == 0 {
			return nil, nil, fmt.Errorf("Expect `}`, but got nothing")
		}
		if rest[0].kind != closeMap {
			return nil, nil, fmt.Errorf("Expect `}`, but got %s", toLiteral(rest))
		}
		return &mapType{t}, rest[1:], nil
	case identifier:
		return &idType{tokens[0].value}, tokens[1:], nil
	default:
		return nil, nil, fmt.Errorf("Unexpected token kind: %v", tokens[0].kind)
	}
}

func (p *typeScriptTypeParser) parseType2(tokens []typeToken) (TypeAst, []typeToken, error) {
	t, tokens2, err := p.parseType1(tokens)
	if err != nil {
		return nil, nil, err
	}
	t2, tokens3 := p.parseArraySuffix(t, tokens2)
	return t2, tokens3, nil
}

func (p *typeScriptTypeParser) parseType3(tokens []typeToken) (TypeAst, []typeToken, error) {
	t, tokens2, err := p.parseType2(tokens)
	if err != nil {
		return nil, nil, err
	}
	return p.parseUnionSuffix(t, tokens2)
}

func (p *typeScriptTypeParser) parseArraySuffix(expr TypeAst, tokens []typeToken) (TypeAst, []typeToken) {
	for len(tokens) > 0 && tokens[0].kind == array {
		expr = &arrayType{expr}
		tokens = tokens[1:]
	}
	return expr, tokens
}

func (p typeScriptTypeParser) parseUnionSuffix(expr TypeAst, tokens []typeToken) (TypeAst, []typeToken, error) {
	exprs := []TypeAst{}

	for len(tokens) > 0 && tokens[0].kind == union {
		extraExpr, rest, err := p.parseType2(tokens[1:])
		if err != nil {
			return nil, nil, err
		}
		exprs = append(exprs, extraExpr)
		tokens = rest
	}

	if len(exprs) == 0 {
		return expr, tokens, nil
	}

	return &unionType{expr, exprs[0], exprs[1:]}, tokens, nil
}
