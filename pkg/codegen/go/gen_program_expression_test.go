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

package gen

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/stretchr/testify/assert"
)

type exprTestCase struct {
	hcl2Expr string
	goCode   string
}

type environment map[string]interface{}

func (e environment) scope() *model.Scope {
	s := model.NewRootScope(syntax.None)
	for name, typeOrFunction := range e {
		switch typeOrFunction := typeOrFunction.(type) {
		case *model.Function:
			s.DefineFunction(name, typeOrFunction)
		case model.Type:
			s.Define(name, &model.Variable{Name: name, VariableType: typeOrFunction})
		}
	}
	return s
}

func TestLiteralExpression(t *testing.T) {
	t.Parallel()

	cases := []exprTestCase{
		{hcl2Expr: "false", goCode: "false"},
		{hcl2Expr: "true", goCode: "true"},
		{hcl2Expr: "0", goCode: "0"},
		{hcl2Expr: "3.14", goCode: "3.14"},
		{hcl2Expr: "\"foo\"", goCode: "\"foo\""},
		{hcl2Expr: `"foo: ${bar}"`, goCode: `fmt.Sprintf("foo: %v", bar)`},
		{hcl2Expr: `"fizz${bar}buzz"`, goCode: `fmt.Sprintf("fizz%vbuzz", bar)`},
		{hcl2Expr: `"foo ${bar} %baz"`, goCode: `fmt.Sprintf("foo %v%v", bar, " %baz")`},
		{hcl2Expr: strings.ReplaceAll(`"{
    \"Version\": \"2008-10-17\",
    \"Statement\": [
        {
            ${Sid}: ${newpolicy},
            ${Effect}: ${Allow},
            \"Principal\": \"*\",
         }
    ]
}"`, "\n", "\\n"), goCode: "fmt.Sprintf(`" + `{
    "Version": "2008-10-17",
    "Statement": [
        {
            %v: %v,
            %v: %v,
            "Principal": "*",
         }
    ]
}` + "`, Sid, newpolicy, Effect, Allow)"},
	}
	for _, c := range cases {
		c := c
		testGenerateExpression(t, c.hcl2Expr, c.goCode, nil, nil)
	}
}

func TestBinaryOpExpression(t *testing.T) {
	t.Parallel()

	env := environment(map[string]interface{}{
		"a": model.BoolType,
		"b": model.BoolType,
		"c": model.NumberType,
		"d": model.NumberType,
	})
	scope := env.scope()

	cases := []exprTestCase{
		{hcl2Expr: "0 == 0", goCode: "0 == 0"},
		{hcl2Expr: "0 != 0", goCode: "0 != 0"},
		{hcl2Expr: "0 < 0", goCode: "0 < 0"},
		{hcl2Expr: "0 > 0", goCode: "0 > 0"},
		{hcl2Expr: "0 <= 0", goCode: "0 <= 0"},
		{hcl2Expr: "0 >= 0", goCode: "0 >= 0"},
		{hcl2Expr: "0 + 0", goCode: "0 + 0"},
		{hcl2Expr: "0 - 0", goCode: "0 - 0"},
		{hcl2Expr: "0 * 0", goCode: "0 * 0"},
		{hcl2Expr: "0 / 0", goCode: "0 / 0"},
		{hcl2Expr: "0 % 0", goCode: "0 % 0"},
		{hcl2Expr: "false && false", goCode: "false && false"},
		{hcl2Expr: "false || false", goCode: "false || false"},
		{hcl2Expr: "a == true", goCode: "a == true"},
		{hcl2Expr: "b == true", goCode: "b == true"},
		{hcl2Expr: "c + 0", goCode: "c + 0"},
		{hcl2Expr: "d + 0", goCode: "d + 0"},
		{hcl2Expr: "a && true", goCode: "a && true"},
		{hcl2Expr: "b && true", goCode: "b && true"},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope, nil)
	}
}

func TestUnaryOpExrepssion(t *testing.T) {
	t.Parallel()

	env := environment(map[string]interface{}{
		"a": model.NumberType,
		"b": model.BoolType,
	})
	scope := env.scope()

	cases := []exprTestCase{
		{hcl2Expr: "-1", goCode: "-1"},
		{hcl2Expr: "!true", goCode: "!true"},
		{hcl2Expr: "-a", goCode: "-a"},
		{hcl2Expr: "!b", goCode: "!b"},
	}

	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope, nil)
	}
}

func TestArgumentTypeName(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))
	noneTypeName := g.argumentTypeName(model.NoneType, false /*isInput*/)
	assert.Equal(t, "", noneTypeName)

	plainIntType := g.argumentTypeName(model.IntType, false /*isInput*/)
	assert.Equal(t, "int", plainIntType)
	inputIntType := g.argumentTypeName(model.IntType, true /*isInput*/)
	assert.Equal(t, "pulumi.Int", inputIntType)

	plainStringType := g.argumentTypeName(model.StringType, false /*isInput*/)
	assert.Equal(t, "string", plainStringType)
	inputStringType := g.argumentTypeName(model.StringType, true /*isInput*/)
	assert.Equal(t, "pulumi.String", inputStringType)

	plainBoolType := g.argumentTypeName(model.BoolType, false /*isInput*/)
	assert.Equal(t, "bool", plainBoolType)
	inputBoolType := g.argumentTypeName(model.BoolType, true /*isInput*/)
	assert.Equal(t, "pulumi.Bool", inputBoolType)

	plainNumberType := g.argumentTypeName(model.NumberType, false /*isInput*/)
	assert.Equal(t, "float64", plainNumberType)
	inputNumberType := g.argumentTypeName(model.NumberType, true /*isInput*/)
	assert.Equal(t, "pulumi.Float64", inputNumberType)

	plainDynamicType := g.argumentTypeName(model.DynamicType, false /*isInput*/)
	assert.Equal(t, "interface{}", plainDynamicType)
	inputDynamicType := g.argumentTypeName(model.DynamicType, true /*isInput*/)
	assert.Equal(t, "pulumi.Any", inputDynamicType)

	objectType := model.NewObjectType(map[string]model.Type{
		"foo": model.StringType,
		"bar": model.IntType,
	})

	plainObjectType := g.argumentTypeName(objectType, false /*isInput*/)
	assert.Equal(t, "map[string]interface{}", plainObjectType)
	inputObjectType := g.argumentTypeName(objectType, true /*isInput*/)
	assert.Equal(t, "pulumi.Map", inputObjectType)

	uniformObjectType := model.NewObjectType(map[string]model.Type{
		"x": model.IntType,
		"y": model.IntType,
	})

	plainUniformObjectType := g.argumentTypeName(uniformObjectType, false /*isInput*/)
	assert.Equal(t, "map[string]interface{}", plainUniformObjectType)
	inputUniformObjectType := g.argumentTypeName(uniformObjectType, true /*isInput*/)
	assert.Equal(t, "pulumi.IntMap", inputUniformObjectType)

	plainMapType := g.argumentTypeName(model.NewMapType(model.StringType), false /*isInput*/)
	assert.Equal(t, "map[string]string", plainMapType)
	inputMapType := g.argumentTypeName(model.NewMapType(model.StringType), true /*isInput*/)
	assert.Equal(t, "pulumi.StringMap", inputMapType)

	plainIntListType := g.argumentTypeName(model.NewListType(model.IntType), false /*isInput*/)
	assert.Equal(t, "[]int", plainIntListType)
	inputIntListType := g.argumentTypeName(model.NewListType(model.IntType), true /*isInput*/)
	assert.Equal(t, "pulumi.IntArray", inputIntListType)

	plainDynamicListType := g.argumentTypeName(model.NewListType(model.DynamicType), false /*isInput*/)
	assert.Equal(t, "[]interface{}", plainDynamicListType)
	inputDynamicListType := g.argumentTypeName(model.NewListType(model.DynamicType), true /*isInput*/)
	assert.Equal(t, "pulumi.Array", inputDynamicListType)

	// assert that the Output[T] + input=false is the same as T + input=true
	// in this case where T = string
	assert.Equal(t,
		g.argumentTypeName(model.NewOutputType(model.StringType), false /*isInput*/),
		g.argumentTypeName(model.StringType, true /*isInput*/))
}

func TestNotYetImplementedEmittedWhenGeneratingFunctions(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))

	notYetImplementedFunctions := []string{
		"split",
		"element",
		"entries",
		"lookup",
		"range",
	}

	for _, fn := range notYetImplementedFunctions {
		var content bytes.Buffer
		g.GenFunctionCallExpression(&content, &model.FunctionCallExpression{
			Name: fn,
		})

		assert.Contains(t, content.String(), "call "+fn)
	}
}

func TestGeneratingGoOptionalFunctions(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))

	testCases := []struct {
		expr      *model.FunctionCallExpression
		generated string
	}{
		{
			expr: &model.FunctionCallExpression{
				Name: "goOptionalString",
				Args: []model.Expression{
					model.VariableReference(&model.Variable{Name: "foo"}),
				},
			},
			generated: "pulumi.StringRef(foo)",
		},
		{
			expr: &model.FunctionCallExpression{
				Name: "goOptionalInt",
				Args: []model.Expression{
					model.VariableReference(&model.Variable{Name: "foo"}),
				},
			},
			generated: "pulumi.IntRef(foo)",
		},
		{
			expr: &model.FunctionCallExpression{
				Name: "goOptionalBool",
				Args: []model.Expression{
					model.VariableReference(&model.Variable{Name: "foo"}),
				},
			},
			generated: "pulumi.BoolRef(foo)",
		},
		{
			expr: &model.FunctionCallExpression{
				Name: "goOptionalFloat64",
				Args: []model.Expression{
					model.VariableReference(&model.Variable{Name: "foo"}),
				},
			},
			generated: "pulumi.Float64Ref(foo)",
		},
	}

	for _, test := range testCases {
		var content bytes.Buffer
		g.GenFunctionCallExpression(&content, test.expr)
		assert.Contains(t, content.String(), test.generated)
	}
}

//nolint:lll
func TestConditionalExpression(t *testing.T) {
	t.Parallel()

	cases := []exprTestCase{
		{
			hcl2Expr: "true ? 1 : 0",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 1\n} else {\ntmp0 = 0\n}\ntmp0",
		},
		{
			hcl2Expr: "true ? 1 : true ? 0 : -1",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 0\n} else {\ntmp0 = -1\n}\nvar tmp1 float64\nif true {\ntmp1 = 1\n} else {\ntmp1 = tmp0\n}\ntmp1",
		},
		{
			hcl2Expr: "true ? true ? 0 : -1 : 0",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 0\n} else {\ntmp0 = -1\n}\nvar tmp1 float64\nif true {\ntmp1 = tmp0\n} else {\ntmp1 = 0\n}\ntmp1",
		},
		{
			hcl2Expr: "{foo = true ? 2 : 0}",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 2\n} else {\ntmp0 = 0\n}\nmap[string]interface{}{\n\"foo\": tmp0,\n}",
		},
	}
	genFunc := func(w io.Writer, g *generator, e model.Expression) {
		e, temps := g.lowerExpression(e, e.Type())
		g.genTemps(w, temps)
		g.Fgenf(w, "%v", e)
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, nil, genFunc)
	}
}

func TestObjectConsExpression(t *testing.T) {
	t.Parallel()

	env := environment(map[string]interface{}{
		"a": model.StringType,
	})
	scope := env.scope()
	cases := []exprTestCase{
		{
			// TODO probably a bug in the binder. Single value objects should just be maps
			hcl2Expr: "{foo = 1}",
			goCode:   "map[string]interface{}{\n\"foo\": 1,\n}",
		},
		{
			hcl2Expr: "{\"foo\" = 1}",
			goCode:   "map[string]interface{}{\n\"foo\": 1,\n}",
		},
		{
			hcl2Expr: "{1 = 1}",
			goCode:   "map[string]interface{}{\n\"1\": 1,\n}",
		},
		{
			hcl2Expr: "{(a) = 1}",
			goCode:   "map[string]float64{\na: 1,\n}",
		},
		{
			hcl2Expr: "{(a+a) = 1}",
			goCode:   "map[string]float64{\na + a: 1,\n}",
		},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope, nil)
	}
}

func TestTupleConsExpression(t *testing.T) {
	t.Parallel()

	env := environment(map[string]interface{}{
		"a": model.StringType,
	})
	scope := env.scope()
	cases := []exprTestCase{
		{
			hcl2Expr: "[\"foo\"]",
			goCode:   "[]string{\n\"foo\",\n}",
		},
		{
			hcl2Expr: "[\"foo\", \"bar\", \"baz\"]",
			goCode:   "[]string{\n\"foo\",\n\"bar\",\n\"baz\",\n}",
		},
		{
			hcl2Expr: "[1]",
			goCode:   "[]float64{\n1,\n}",
		},
		{
			hcl2Expr: "[1,2,3]",
			goCode:   "[]float64{\n1,\n2,\n3,\n}",
		},
		{
			hcl2Expr: "[1,\"foo\"]",
			goCode:   "[]interface{}{\n1,\n\"foo\",\n}",
		},
	}
	for _, c := range cases {
		c := c
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope, nil)
	}
}

func testGenerateExpression(
	t *testing.T,
	hcl2Expr, goCode string,
	scope *model.Scope,
	gen func(w io.Writer, g *generator, e model.Expression),
) {
	t.Run(hcl2Expr, func(t *testing.T) {
		t.Parallel()

		// test program is only for schema info
		g := newTestGenerator(t, filepath.Join("aws-s3-logging-pp", "aws-s3-logging.pp"))
		var index bytes.Buffer
		expr, _ := model.BindExpressionText(hcl2Expr, scope, hcl.Pos{})
		if gen != nil {
			gen(&index, g, expr)
		} else {
			g.Fgenf(&index, "%v", expr)
		}

		assert.Equal(t, goCode, index.String())
	})
}
