// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func assertConvertibleFrom(t *testing.T, to, from Type) {
	assert.NotEqual(t, NoConversion, to.ConversionFrom(from))
}

func TestBindLiteral(t *testing.T) {
	expr, diags := BindExpressionText("false", nil, hcl.Pos{})
	assert.Len(t, diags, 0)
	assertConvertibleFrom(t, BoolType, expr.Type())
	lit, ok := expr.(*LiteralValueExpression)
	assert.True(t, ok)
	assert.Equal(t, cty.False, lit.Value)
	assert.Equal(t, "false", fmt.Sprintf("%v", expr))

	expr, diags = BindExpressionText("true", nil, hcl.Pos{})
	assert.Len(t, diags, 0)
	assertConvertibleFrom(t, BoolType, expr.Type())
	lit, ok = expr.(*LiteralValueExpression)
	assert.True(t, ok)
	assert.Equal(t, cty.True, lit.Value)
	assert.Equal(t, "true", fmt.Sprintf("%v", expr))

	expr, diags = BindExpressionText("0", nil, hcl.Pos{})
	assert.Len(t, diags, 0)
	assertConvertibleFrom(t, NumberType, expr.Type())
	lit, ok = expr.(*LiteralValueExpression)
	assert.True(t, ok)
	assert.True(t, cty.NumberIntVal(0).RawEquals(lit.Value))
	assert.Equal(t, "0", fmt.Sprintf("%v", expr))

	expr, diags = BindExpressionText("3.14", nil, hcl.Pos{})
	assert.Len(t, diags, 0)
	assertConvertibleFrom(t, NumberType, expr.Type())
	lit, ok = expr.(*LiteralValueExpression)
	assert.True(t, ok)
	assert.True(t, cty.MustParseNumberVal("3.14").RawEquals(lit.Value))
	assert.Equal(t, "3.14", fmt.Sprintf("%v", expr))

	expr, diags = BindExpressionText(`"foo"`, nil, hcl.Pos{})
	assert.Len(t, diags, 0)
	assertConvertibleFrom(t, StringType, expr.Type())
	template, ok := expr.(*TemplateExpression)
	assert.True(t, ok)
	assert.Len(t, template.Parts, 1)
	lit, ok = template.Parts[0].(*LiteralValueExpression)
	assert.True(t, ok)
	assert.Equal(t, cty.StringVal("foo"), lit.Value)
	assert.Equal(t, "\"foo\"", fmt.Sprintf("%v", expr))
}

type environment map[string]interface{}

func (e environment) scope() *Scope {
	s := NewRootScope(syntax.None)
	for name, typeOrFunction := range e {
		switch typeOrFunction := typeOrFunction.(type) {
		case *Function:
			s.DefineFunction(name, typeOrFunction)
		case Type:
			s.Define(name, &Variable{Name: name, VariableType: typeOrFunction})
		}
	}
	return s
}

type exprTestCase struct {
	x  string
	t  Type
	xt Expression
}

func TestBindBinaryOp(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": NewOutputType(BoolType),
		"b": NewPromiseType(BoolType),
		"c": NewOutputType(NumberType),
		"d": NewPromiseType(NumberType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Comparisons
		{x: "0 == 0", t: BoolType},
		{x: "0 != 0", t: BoolType},
		{x: "0 < 0", t: BoolType},
		{x: "0 > 0", t: BoolType},
		{x: "0 <= 0", t: BoolType},
		{x: "0 >= 0", t: BoolType},

		// Arithmetic
		{x: "0 + 0", t: NumberType},
		{x: "0 - 0", t: NumberType},
		{x: "0 * 0", t: NumberType},
		{x: "0 / 0", t: NumberType},
		{x: "0 % 0", t: NumberType},

		// Logical
		{x: "false && false", t: BoolType},
		{x: "false || false", t: BoolType},

		// Lifted operations
		{x: "a == true", t: NewOutputType(BoolType)},
		{x: "b == true", t: NewPromiseType(BoolType)},
		{x: "c + 0", t: NewOutputType(NumberType)},
		{x: "d + 0", t: NewPromiseType(NumberType)},
		{x: "a && true", t: NewOutputType(BoolType)},
		{x: "b && true", t: NewPromiseType(BoolType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*BinaryOpExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindConditional(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": NewOutputType(BoolType),
		"b": NewPromiseType(BoolType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		{x: "true ? 0 : 1", t: NumberType},
		{x: "true ? 0 : false", t: NewUnionType(NumberType, BoolType)},
		{x: "true ? a : b", t: NewOutputType(BoolType)},

		// Lifted operations
		{x: "a ? 0 : 1", t: NewOutputType(NumberType)},
		{x: "b ? 0 : 1", t: NewPromiseType(NumberType)},
		{x: "a ? 0 : false", t: NewOutputType(NewUnionType(NumberType, BoolType))},
		{x: "b ? 0 : false", t: NewPromiseType(NewUnionType(NumberType, BoolType))},
		{x: "a ? a : b", t: NewOutputType(BoolType)},
		{x: "b ? b : b", t: NewPromiseType(BoolType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*ConditionalExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindFor(t *testing.T) {
	// TODO: union collection types

	env := environment(map[string]interface{}{
		"a":  NewMapType(StringType),
		"aa": NewMapType(NewOutputType(StringType)),
		"b":  NewOutputType(NewMapType(StringType)),
		"c":  NewPromiseType(NewMapType(StringType)),
		"d":  NewListType(StringType),
		"dd": NewListType(NewOutputType(StringType)),
		"e":  NewOutputType(NewListType(StringType)),
		"f":  NewPromiseType(NewListType(StringType)),
		"g":  BoolType,
		"h":  NewOutputType(BoolType),
		"i":  NewPromiseType(BoolType),
		"j":  StringType,
		"k":  NewOutputType(StringType),
		"l":  NewPromiseType(StringType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Object for
		{x: `{for k, v in {}: k => v}`, t: NewMapType(NoneType)},
		{x: `{for k, v in {foo = "bar"}: k => v}`, t: NewMapType(StringType)},
		{x: `{for k, v in {foo = "bar"}: k => 0}`, t: NewMapType(NumberType)},
		{x: `{for k, v in {foo = 0}: k => v}`, t: NewMapType(NumberType)},
		{x: `{for k, v in a: k => v}`, t: NewMapType(StringType)},
		{x: `{for k, v in aa: k => v}`, t: NewMapType(NewOutputType(StringType))},
		{x: `{for k, v in a: k => 0}`, t: NewMapType(NumberType)},
		{x: `{for k, v in d: v => k}`, t: NewMapType(NumberType)},
		{x: `{for k, v in d: v => k...}`, t: NewMapType(NewListType(NumberType))},
		{x: `{for k, v in d: v => k if k > 10}`, t: NewMapType(NumberType)},

		// List for
		{x: `[for k, v in {}: [k, v]]`, t: NewListType(NewTupleType(StringType, NoneType))},
		{x: `[for k, _ in {}: k]`, t: NewListType(StringType)},
		{x: `[for v in []: v]`, t: NewListType(NoneType)},

		// Lifted operations
		{x: `{for k, v in b: k => v}`, t: NewOutputType(NewMapType(StringType))},
		{x: `{for k, v in c: k => v}`, t: NewPromiseType(NewMapType(StringType))},
		{x: `{for k, v in {}: k => v if h}`, t: NewOutputType(NewMapType(NoneType))},
		{x: `{for k, v in {}: k => v if i}`, t: NewPromiseType(NewMapType(NoneType))},
		{x: `[for v in e: v]`, t: NewOutputType(NewListType(StringType))},
		{x: `[for v in f: v]`, t: NewPromiseType(NewListType(StringType))},
		{x: `[for v in []: v if h]`, t: NewOutputType(NewListType(NoneType))},
		{x: `[for v in []: v if i]`, t: NewPromiseType(NewListType(NoneType))},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*ForExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindFunctionCall(t *testing.T) {
	env := environment(map[string]interface{}{
		"f0": NewFunction(StaticFunctionSignature{
			Parameters: []Parameter{
				{Name: "foo", Type: StringType},
				{Name: "bar", Type: IntType},
			},
			ReturnType: BoolType,
		}),
		"f1": NewFunction(StaticFunctionSignature{
			Parameters: []Parameter{
				{Name: "foo", Type: StringType},
			},
			VarargsParameter: &Parameter{
				Name: "bar", Type: IntType,
			},
			ReturnType: BoolType,
		}),
		"a": NewOutputType(StringType),
		"b": NewPromiseType(StringType),
		"c": NewOutputType(IntType),
		"d": NewPromiseType(IntType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Standard calls
		{x: `f0("foo", 0)`, t: BoolType},
		{x: `f1("foo")`, t: BoolType},
		{x: `f1("foo", 1, 2, 3)`, t: BoolType},

		// Lifted calls
		{x: `f0(a, 0)`, t: NewOutputType(BoolType)},
		{x: `f0(b, 0)`, t: NewPromiseType(BoolType)},
		{x: `f0("foo", c)`, t: NewOutputType(BoolType)},
		{x: `f0("foo", d)`, t: NewPromiseType(BoolType)},
		{x: `f0(a, d)`, t: NewOutputType(BoolType)},
		{x: `f0(b, c)`, t: NewOutputType(BoolType)},
		{x: `f1(a)`, t: NewOutputType(BoolType)},
		{x: `f1(b)`, t: NewPromiseType(BoolType)},
		{x: `f1("foo", c)`, t: NewOutputType(BoolType)},
		{x: `f1("foo", d)`, t: NewPromiseType(BoolType)},
		{x: `f1("foo", c, d)`, t: NewOutputType(BoolType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*FunctionCallExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindIndex(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": StringType,
		"b": IntType,
		"c": NewOutputType(StringType),
		"d": NewOutputType(IntType),
		"e": NewPromiseType(StringType),
		"f": NewPromiseType(IntType),
		"g": NewListType(BoolType),
		"h": NewMapType(BoolType),
		"i": NewObjectType(map[string]Type{"foo": BoolType}),
		"j": NewOutputType(NewListType(BoolType)),
		"k": NewOutputType(NewMapType(BoolType)),
		"l": NewOutputType(NewObjectType(map[string]Type{"foo": BoolType})),
		"m": NewPromiseType(NewListType(BoolType)),
		"n": NewPromiseType(NewMapType(BoolType)),
		"o": NewPromiseType(NewObjectType(map[string]Type{"foo": BoolType})),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Standard operations
		{x: "g[a]", t: BoolType},
		{x: "g[b]", t: BoolType},
		{x: "h[a]", t: BoolType},
		{x: "h[b]", t: BoolType},
		{x: "i[a]", t: BoolType},
		{x: "i[b]", t: BoolType},

		// Lifted operations
		{x: "g[c]", t: NewOutputType(BoolType)},
		{x: "g[d]", t: NewOutputType(BoolType)},
		{x: "h[c]", t: NewOutputType(BoolType)},
		{x: "h[d]", t: NewOutputType(BoolType)},
		{x: "i[c]", t: NewOutputType(BoolType)},
		{x: "i[d]", t: NewOutputType(BoolType)},
		{x: "g[e]", t: NewPromiseType(BoolType)},
		{x: "g[f]", t: NewPromiseType(BoolType)},
		{x: "h[e]", t: NewPromiseType(BoolType)},
		{x: "h[f]", t: NewPromiseType(BoolType)},
		{x: "i[e]", t: NewPromiseType(BoolType)},
		{x: "i[f]", t: NewPromiseType(BoolType)},
		{x: "j[a]", t: NewOutputType(BoolType)},
		{x: "j[b]", t: NewOutputType(BoolType)},
		{x: "k[a]", t: NewOutputType(BoolType)},
		{x: "k[b]", t: NewOutputType(BoolType)},
		{x: "l[a]", t: NewOutputType(BoolType)},
		{x: "l[b]", t: NewOutputType(BoolType)},
		{x: "m[a]", t: NewPromiseType(BoolType)},
		{x: "m[b]", t: NewPromiseType(BoolType)},
		{x: "n[a]", t: NewPromiseType(BoolType)},
		{x: "n[b]", t: NewPromiseType(BoolType)},
		{x: "o[a]", t: NewPromiseType(BoolType)},
		{x: "o[b]", t: NewPromiseType(BoolType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*IndexExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindObjectCons(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": StringType,
		"b": NumberType,
		"c": BoolType,
		"d": NewOutputType(StringType),
		"e": NewOutputType(NumberType),
		"f": NewOutputType(BoolType),
		"g": NewPromiseType(StringType),
		"h": NewPromiseType(NumberType),
		"i": NewPromiseType(BoolType),
	})
	scope := env.scope()

	ot := NewObjectType(map[string]Type{"foo": StringType, "0": NumberType, "false": BoolType})
	mt := NewMapType(StringType)
	cases := []exprTestCase{
		// Standard operations
		{x: `{"foo": "oof", 0: 42, false: true}`, t: ot},
		{x: `{(a): a, (b): b, (c): c}`, t: mt},

		// Lifted operations
		{x: `{(d): a, (b): b, (c): c}`, t: NewOutputType(mt)},
		{x: `{(a): a, (e): b, (c): c}`, t: NewOutputType(mt)},
		{x: `{(a): a, (b): b, (f): c}`, t: NewOutputType(mt)},
		{x: `{(g): a, (b): b, (c): c}`, t: NewPromiseType(mt)},
		{x: `{(a): a, (h): b, (c): c}`, t: NewPromiseType(mt)},
		{x: `{(a): a, (b): b, (i): c}`, t: NewPromiseType(mt)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*ObjectConsExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindRelativeTraversal(t *testing.T) {
	env := environment(map[string]interface{}{
		"a":  NewMapType(StringType),
		"aa": NewMapType(NewOutputType(StringType)),
		"b":  NewOutputType(NewMapType(StringType)),
		"c":  NewPromiseType(NewMapType(StringType)),
		"d":  NewListType(StringType),
		"dd": NewListType(NewOutputType(StringType)),
		"e":  NewOutputType(NewListType(StringType)),
		"f":  NewPromiseType(NewListType(StringType)),
		"g":  BoolType,
		"h":  NewOutputType(BoolType),
		"i":  NewPromiseType(BoolType),
		"j":  StringType,
		"k":  NewOutputType(StringType),
		"l":  NewPromiseType(StringType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Object for
		{x: `{for k, v in {foo: "bar"}: k => v}.foo`, t: StringType},
		{x: `{for k, v in {foo: "bar"}: k => 0}.foo`, t: NumberType},
		{x: `{for k, v in {foo: 0}: k => v}.foo`, t: NumberType},
		{x: `{for k, v in a: k => v}.foo`, t: StringType},
		{x: `{for k, v in aa: k => v}.foo`, t: NewOutputType(StringType)},
		{x: `{for k, v in a: k => 0}.foo`, t: NumberType},
		{x: `{for k, v in d: v => k}.foo`, t: NumberType},
		{x: `{for k, v in d: v => k...}.foo[0]`, t: NumberType},
		{x: `{for k, v in d: v => k if k > 10}.foo`, t: NumberType},

		// List for
		{x: `[for k, v in {}: [k, v]].0`, t: NewTupleType(StringType, NoneType)},
		{x: `[for k, _ in {}: k].0`, t: StringType},

		// Lifted operations
		{x: `{for k, v in b: k => v}.foo`, t: NewOutputType(StringType)},
		{x: `{for k, v in c: k => v}.foo`, t: NewPromiseType(StringType)},
		{x: `[for v in e: v].foo`, t: NewOutputType(StringType)},
		{x: `[for v in f: v].foo`, t: NewPromiseType(StringType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*RelativeTraversalExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindScopeTraversal(t *testing.T) {
	ot := NewObjectType(map[string]Type{
		"foo": NewListType(StringType),
		"bar": NewObjectType(map[string]Type{
			"baz": StringType,
		}),
	})
	env := environment(map[string]interface{}{
		"a": StringType,
		"b": IntType,
		"c": NewListType(BoolType),
		"d": NewMapType(BoolType),
		"e": ot,
		"f": NewOutputType(StringType),
		"g": NewOutputType(IntType),
		"h": NewOutputType(NewListType(BoolType)),
		"i": NewOutputType(NewMapType(BoolType)),
		"j": NewOutputType(ot),
		"k": NewPromiseType(StringType),
		"l": NewPromiseType(IntType),
		"m": NewPromiseType(NewListType(BoolType)),
		"n": NewPromiseType(NewMapType(BoolType)),
		"o": NewPromiseType(ot),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Standard traversals
		{x: `a`, t: StringType},
		{x: `b`, t: IntType},
		{x: `c`, t: NewListType(BoolType)},
		{x: `d`, t: NewMapType(BoolType)},
		{x: `e`, t: ot},
		{x: `f`, t: NewOutputType(StringType)},
		{x: `g`, t: NewOutputType(IntType)},
		{x: `k`, t: NewPromiseType(StringType)},
		{x: `l`, t: NewPromiseType(IntType)},
		{x: `c.0`, t: BoolType},
		{x: `d.foo`, t: BoolType},
		{x: `e.foo`, t: NewListType(StringType)},
		{x: `e.foo.0`, t: StringType},
		{x: `e.bar`, t: ot.Properties["bar"]},
		{x: `e.bar.baz`, t: StringType},

		// Lifted traversals
		{x: `h.0`, t: NewOutputType(BoolType)},
		{x: `i.foo`, t: NewOutputType(BoolType)},
		{x: `j.foo`, t: NewOutputType(NewListType(StringType))},
		{x: `j.foo.0`, t: NewOutputType(StringType)},
		{x: `j.bar`, t: NewOutputType(ot.Properties["bar"])},
		{x: `j.bar.baz`, t: NewOutputType(StringType)},
		{x: `m.0`, t: NewPromiseType(BoolType)},
		{x: `n.foo`, t: NewPromiseType(BoolType)},
		{x: `o.foo`, t: NewPromiseType(NewListType(StringType))},
		{x: `o.foo.0`, t: NewPromiseType(StringType)},
		{x: `o.bar`, t: NewPromiseType(ot.Properties["bar"])},
		{x: `o.bar.baz`, t: NewPromiseType(StringType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*ScopeTraversalExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindSplat(t *testing.T) {
	ot := NewObjectType(map[string]Type{
		"foo": NewListType(StringType),
		"bar": NewObjectType(map[string]Type{
			"baz": StringType,
		}),
	})
	env := environment(map[string]interface{}{
		"a": NewListType(NewListType(StringType)),
		"b": NewListType(ot),
		"c": NewSetType(NewListType(StringType)),
		"d": NewSetType(ot),
		//		"e": NewTupleType(NewListType(StringType)),
		//		"f": NewTupleType(ot),
		"g": NewListType(NewListType(NewOutputType(StringType))),
		"h": NewListType(NewListType(NewPromiseType(StringType))),
		//		"i": NewSetType(NewListType(NewOutputType(StringType))),
		//		"j": NewSetType(NewListType(NewPromiseType(StringType))),
		//		"k": NewTupleType(NewListType(NewOutputType(StringType))),
		//		"l": NewTupleType(NewListType(NewPromiseType(StringType))),
		"m": NewOutputType(NewListType(ot)),
		"n": NewPromiseType(NewListType(ot)),
		//		"o": NewOutputType(NewSetType(ot)),
		//		"p": NewPromiseType(NewSetType(ot)),
		//		"q": NewOutputType(NewTupleType(ot)),
		//		"r": NewPromiseType(NewTupleType(ot)),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Standard operations
		{x: `a[*][0]`, t: NewListType(StringType)},
		{x: `b[*].bar.baz`, t: NewListType(StringType)},
		{x: `b.*.bar.baz`, t: NewListType(StringType)},
		//		{x: `c[*][0]`, t: NewSetType(StringType)},
		//		{x: `d[*].bar.baz`, t: NewSetType(StringType)},
		//		{x: `e[*][0]`, t: NewTupleType(StringType)},
		//		{x: `f[*].bar.baz`, t: NewTupleType(StringType)},
		{x: `g[*][0]`, t: NewListType(NewOutputType(StringType))},
		{x: `h[*][0]`, t: NewListType(NewPromiseType(StringType))},
		//		{x: `i[*][0]`, t: NewListType(NewOutputType(StringType))},
		//		{x: `j[*][0]`, t: NewListType(NewPromiseType(StringType))},
		//		{x: `k[*][0]`, t: NewTupleType(NewOutputType(StringType))},
		//		{x: `l[*][0]`, t: NewTupleType(NewPromiseType(StringType))},

		// Lifted operations
		{x: `m[*].bar.baz`, t: NewOutputType(NewListType(StringType))},
		{x: `n[*].bar.baz`, t: NewPromiseType(NewListType(StringType))},
		{x: `m.*.bar.baz`, t: NewOutputType(NewListType(StringType))},
		{x: `n.*.bar.baz`, t: NewPromiseType(NewListType(StringType))},
		//		{x: `o[*].bar.baz`, t: NewOutputType(NewListType(StringType))},
		//		{x: `p[*].bar.baz`, t: NewPromiseType(NewListType(StringType))},
		//		{x: `q[*].bar.baz`, t: NewOutputType(NewTupleType(StringType))},
		//		{x: `r[*].bar.baz`, t: NewPromiseType(NewTupleType(StringType))},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*SplatExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindTemplate(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": StringType,
		"b": NumberType,
		"c": BoolType,
		"d": NewListType(StringType),
		"e": NewOutputType(StringType),
		"f": NewOutputType(NumberType),
		"g": NewOutputType(BoolType),
		"h": NewOutputType(NewListType(StringType)),
		"i": NewPromiseType(StringType),
		"j": NewPromiseType(NumberType),
		"k": NewPromiseType(BoolType),
		"l": NewPromiseType(NewListType(StringType)),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Unwrapped interpolations
		{x: `"${0}"`, t: NumberType, xt: &LiteralValueExpression{}},
		{x: `"${true}"`, t: BoolType, xt: &LiteralValueExpression{}},
		{x: `"${d}"`, t: NewListType(StringType), xt: &ScopeTraversalExpression{}},
		{x: `"${e}"`, t: NewOutputType(StringType), xt: &ScopeTraversalExpression{}},
		{x: `"${i}"`, t: NewPromiseType(StringType), xt: &ScopeTraversalExpression{}},

		// Simple interpolations
		{x: `"v: ${a}"`, t: StringType},
		{x: `"v: ${b}"`, t: StringType},
		{x: `"v: ${c}"`, t: StringType},
		{x: `"v: ${d}"`, t: StringType},

		// Template control expressions
		{x: `"%{if c} v: ${a} %{endif}"`, t: StringType},
		{x: `"%{for v in d} v: ${v} %{endfor}"`, t: StringType},

		// Lifted operations
		{x: `"v: ${e}"`, t: NewOutputType(StringType)},
		{x: `"v: ${f}"`, t: NewOutputType(StringType)},
		{x: `"v: ${g}"`, t: NewOutputType(StringType)},
		{x: `"v: ${h}"`, t: NewOutputType(StringType)},
		{x: `"%{if g} v: ${a} %{endif}"`, t: NewOutputType(StringType)},
		{x: `"%{for v in h} v: ${v} %{endfor}"`, t: NewOutputType(StringType)},
		{x: `"v: ${i}"`, t: NewPromiseType(StringType)},
		{x: `"v: ${j}"`, t: NewPromiseType(StringType)},
		{x: `"v: ${k}"`, t: NewPromiseType(StringType)},
		{x: `"v: ${l}"`, t: NewPromiseType(StringType)},
		{x: `"%{if k} v: ${a} %{endif}"`, t: NewPromiseType(StringType)},
		{x: `"%{for v in l} v: ${v} %{endfor}"`, t: NewPromiseType(StringType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())

			var ok bool
			switch c.xt.(type) {
			case *LiteralValueExpression:
				_, ok = expr.(*LiteralValueExpression)
			case *ScopeTraversalExpression:
				_, ok = expr.(*ScopeTraversalExpression)
			default:
				_, ok = expr.(*TemplateExpression)
				assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
			}
			assert.True(t, ok)
		})
	}
}

func TestBindTupleCons(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": NewOutputType(StringType),
		"b": NewPromiseType(StringType),
		"c": NewUnionType(StringType, BoolType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		{x: `["foo", "bar", "baz"]`, t: NewTupleType(StringType, StringType, StringType)},
		{x: `[0, "foo", true]`, t: NewTupleType(NumberType, StringType, BoolType)},
		{x: `[a, b, c]`, t: NewTupleType(env["a"].(Type), env["b"].(Type), env["c"].(Type))},
		{x: `[{"foo": "bar"}]`, t: NewTupleType(NewObjectType(map[string]Type{"foo": StringType}))},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*TupleConsExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}

func TestBindUnaryOp(t *testing.T) {
	env := environment(map[string]interface{}{
		"a": NumberType,
		"b": BoolType,
		"c": NewOutputType(NumberType),
		"d": NewOutputType(BoolType),
		"e": NewPromiseType(NumberType),
		"f": NewPromiseType(BoolType),
	})
	scope := env.scope()

	cases := []exprTestCase{
		// Standard operations
		{x: `-a`, t: NumberType},
		{x: `!b`, t: BoolType},

		// Lifted operations
		{x: `-c`, t: NewOutputType(NumberType)},
		{x: `-e`, t: NewPromiseType(NumberType)},
		{x: `!d`, t: NewOutputType(BoolType)},
		{x: `!f`, t: NewPromiseType(BoolType)},
	}
	for _, c := range cases {
		t.Run(c.x, func(t *testing.T) {
			expr, diags := BindExpressionText(c.x, scope, hcl.Pos{})
			assert.Len(t, diags, 0)
			assertConvertibleFrom(t, c.t, expr.Type())
			_, ok := expr.(*UnaryOpExpression)
			assert.True(t, ok)
			assert.Equal(t, c.x, fmt.Sprintf("%v", expr))
		})
	}
}
