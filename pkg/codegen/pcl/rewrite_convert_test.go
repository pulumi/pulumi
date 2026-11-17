// Copyright 2020, Pulumi Corporation.
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

package pcl

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteConversions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input, output string
		to            model.Type
	}{
		{
			input:  `"1.5" + 2.5`,
			output: `1.5 + 2.5`,
		},
		{
			input:  `"1" + 2`,
			output: `1 + 2`,
		},
		{
			input:  `-"1"`,
			output: `-1`,
		},
		{
			input:  `"1" + 2.5`,
			output: `__convert(1) + 2.5`,
		},
		{
			input:  `-"1.5"`,
			output: `-1.5`,
		},
		{
			input:  `{a: "b"}`,
			output: `{a: "b"}`,
			to: model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			}),
		},
		{
			input:  `{a: "b"}`,
			output: `{a: "b"}`,
			to: model.InputType(model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			})),
		},
		{
			input:  `{a: "b"}`,
			output: `__convert({a: "b"})`,
			to: model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			}, &schema.ObjectType{}),
		},
		{
			input:  `{a: "b"}`,
			output: `__convert({a: "b"})`,
			to: model.InputType(model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			}, &schema.ObjectType{})),
		},
		{
			input:  `{a: "1.5" + 2.5}`,
			output: `{a: 1.5 + 2.5}`,
			to: model.NewObjectType(map[string]model.Type{
				"a": model.NumberType,
			}),
		},
		{
			input:  `[{a: "b"}]`,
			output: "__convert([\n    __convert({a: \"b\"})])",
			to: model.NewListType(model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			}, &schema.ObjectType{})),
		},
		{
			input:  `[for v in ["b"]: {a: v}]`,
			output: `[for v in ["b"]: __convert( {a: v})]`,
			to: model.NewListType(model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			}, &schema.ObjectType{})),
		},
		{
			input:  `true ? {a: "b"} : {a: "c"}`,
			output: `true ? __convert( {a: "b"}) : __convert( {a: "c"})`,
			to: model.NewObjectType(map[string]model.Type{
				"a": model.StringType,
			}, &schema.ObjectType{}),
		},
		{
			input:  `!"true"`,
			output: `!true`,
			to:     model.BoolType,
		},
		{
			input:  `["a"][i]`,
			output: `["a"][__convert(i)]`,
			to:     model.StringType,
		},
		{
			input:  `42.5`,
			output: `__convert(42.5)`,
			to:     model.IntType,
		},
		{
			input:  `"42.5"`,
			output: `__convert(42.5)`,
			to:     model.IntType,
		},
		{
			input:  `{a: 42.5}`,
			output: `{a: __convert( 42.5)}`,
			to: model.NewObjectType(map[string]model.Type{
				"a": model.IntType,
			}),
		},
		{
			input:  `outString`,
			output: `__convert(outString)`,
			to:     model.NewOutputType(model.NumberType),
		},
		{
			input:  `outString`,
			output: `outString`,
			to:     model.NewOutputType(model.StringType),
		},
	}

	scope := model.NewRootScope(syntax.None)
	scope.Define("i", &model.Variable{
		Name:         "i",
		VariableType: model.StringType,
	})
	scope.Define("outString", &model.Variable{
		Name:         "outString",
		VariableType: model.NewOutputType(model.StringType),
	})
	for _, c := range cases {
		expr, diags := model.BindExpressionText(c.input, scope, hcl.Pos{})
		require.Len(t, diags, 0)

		to := c.to
		if to == nil {
			to = expr.Type()
		}
		expr, diags = RewriteConversions(expr, to)
		require.Len(t, diags, 0)
		assert.Equal(t, c.output, fmt.Sprintf("%v", expr))
	}
}

func TestRewriteConversionsAfterApply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input, output string
	}{
		{
			input:  `f({id: v.id})`,
			output: `__apply(v,eval(v, f(__convert({id: v.id}))))`,
		},
	}

	scope := model.NewRootScope(syntax.None)
	scope.DefineFunction("f", model.NewFunction(model.StaticFunctionSignature{
		Parameters: []model.Parameter{{
			Name: "args",
			Type: model.NewObjectType(map[string]model.Type{
				"id": model.StringType,
			}, &schema.ObjectType{}),
		}},
		ReturnType: model.DynamicType,
	}))
	scope.Define("v", &model.Variable{
		Name: "v",
		VariableType: model.NewOutputType(model.NewObjectType(map[string]model.Type{
			"id": model.StringType,
		})),
	})

	for _, c := range cases {
		expr, diags := model.BindExpressionText(c.input, scope, hcl.Pos{})
		require.Len(t, diags, 0)

		expr, _ = RewriteApplies(expr, nameInfo(0), false)
		expr, diags = RewriteConversions(expr, expr.Type())
		require.Len(t, diags, 0)
		assert.Equal(t, c.output, fmt.Sprintf("%v", expr))
	}
}

func TestRewriteConversionsExpandFinal(t *testing.T) {
	t.Parallel()

	scope := model.NewRootScope(syntax.None)
	scope.DefineFunction("max", model.NewFunction(model.StaticFunctionSignature{
		Parameters: []model.Parameter{{
			Name: "first",
			Type: model.IntType,
		}},
		VarargsParameter: &model.Parameter{
			Name: "rest",
			Type: model.IntType,
		},
		ReturnType: model.IntType,
	}))

	expr, diags := model.BindExpressionText(`max(0, [1, 2, 3]...)`, scope, hcl.Pos{})
	require.Empty(t, diags)

	expr, diags = RewriteConversions(expr, expr.Type())
	require.Empty(t, diags)
	call := expr.(*model.FunctionCallExpression)
	assert.True(t, call.ExpandFinal)
	require.IsType(t, &model.TupleConsExpression{}, call.Args[1])
}

// Tests that LowerConversion picks a member of a union destination without regard to the order of the members: a
// source of unknown type is left as it is, none is never a target, and a plain member is preferred to an eventual
// one when only unsafe conversions exist.
func TestLowerConversionIsOrderIndependent(t *testing.T) {
	t.Parallel()

	variable := func(name string, typ model.Type) model.Expression {
		return model.VariableReference(&model.Variable{Name: name, VariableType: typ})
	}
	optionalString := model.NewOptionalType(model.StringType)
	cases := []struct {
		name    string
		from    model.Expression
		members []model.Type
		want    model.Type
	}{
		{
			name:    "unknown source is left alone",
			from:    variable("x", model.DynamicType),
			members: []model.Type{model.StringType, model.NoneType, model.NewOutputType(model.StringType)},
			want:    nil,
		},
		{
			name:    "optional source skips none",
			from:    variable("x", optionalString),
			members: []model.Type{model.StringType, model.NoneType, model.NewOutputType(model.StringType)},
			want:    model.StringType,
		},
		{
			name:    "plain member preferred to eventual",
			from:    variable("x", model.StringType),
			members: []model.Type{model.NumberType, model.NewOutputType(model.NumberType)},
			want:    model.NumberType,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, members := range [][]model.Type{c.members, reversed(c.members)} {
				union := &model.UnionType{ElementTypes: members}
				got := LowerConversion(c.from, union)
				want := c.want
				if want == nil {
					want = union
				}
				assert.True(t, want.Equals(got), "members %v: expected %v, got %v", members, want, got)
			}
		})
	}
}

func reversed(types []model.Type) []model.Type {
	out := slices.Clone(types)
	slices.Reverse(out)
	return out
}
