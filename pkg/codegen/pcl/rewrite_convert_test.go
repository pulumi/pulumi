// Copyright 2020-2024, Pulumi Corporation.
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
			input:  `"1" + 2`,
			output: `1 + 2`,
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
			input:  `{a: "1" + 2}`,
			output: `{a: 1 + 2}`,
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
			input:  `42`,
			output: `__convert(42)`,
			to:     model.IntType,
		},
		{
			input:  `"42"`,
			output: `__convert(42)`,
			to:     model.IntType,
		},
		{
			input:  `{a: 42}`,
			output: `{a: __convert( 42)}`,
			to: model.NewObjectType(map[string]model.Type{
				"a": model.IntType,
			}),
		},
	}

	scope := model.NewRootScope(syntax.None)
	scope.Define("i", &model.Variable{
		Name:         "i",
		VariableType: model.StringType,
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
