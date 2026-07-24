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
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exprTestCase struct {
	hcl2Expr string
	goCode   string
}

type environment map[string]any

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
		testGenerateExpression(t, c.hcl2Expr, c.goCode, nil, nil)
	}
}

func TestBinaryOpExpression(t *testing.T) {
	t.Parallel()

	env := environment(map[string]any{
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

	env := environment(map[string]any{
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

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))
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

	// Asset and Archive opaque types must be qualified with the pulumi package, otherwise nested compositions
	// render bare names like []AssetMap which is invalid Go.
	assert.Equal(t, "pulumi.AssetOrArchive", g.argumentTypeName(pcl.AssetType, false /*isInput*/))
	assert.Equal(t, "pulumi.AssetOrArchive", g.argumentTypeName(pcl.AssetType, true /*isInput*/))
	assert.Equal(t, "pulumi.Archive", g.argumentTypeName(pcl.ArchiveType, false /*isInput*/))
	assert.Equal(t, "pulumi.Archive", g.argumentTypeName(pcl.ArchiveType, true /*isInput*/))
	assert.Equal(t,
		"pulumi.AssetOrArchiveArrayMap",
		g.argumentTypeName(
			model.NewObjectType(map[string]model.Type{"k": model.NewListType(pcl.AssetType)}),
			true, /*isInput*/
		))

	// assert that the Output[T] + input=false is the same as T + input=true
	// in this case where T = string
	assert.Equal(t,
		g.argumentTypeName(model.NewOutputType(model.StringType), false /*isInput*/),
		g.argumentTypeName(model.StringType, true /*isInput*/))
}

func TestNotYetImplementedEmittedWhenGeneratingFunctions(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))

	notYetImplementedFunctions := []string{
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

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))

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
			hcl2Expr: "true ? 1.5 : 0.5",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 1.5\n} else {\ntmp0 = 0.5\n}\ntmp0",
		},
		{
			hcl2Expr: "true ? 1.5 : true ? 0.5 : -1.5",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 0.5\n} else {\ntmp0 = -1.5\n}\nvar tmp1 float64\nif true {\ntmp1 = 1.5\n} else {\ntmp1 = tmp0\n}\ntmp1",
		},
		{
			hcl2Expr: "true ? true ? 0.5 : -1.5 : 0.5",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 0.5\n} else {\ntmp0 = -1.5\n}\nvar tmp1 float64\nif true {\ntmp1 = tmp0\n} else {\ntmp1 = 0.5\n}\ntmp1",
		},
		{
			hcl2Expr: "{foo = true ? 2.5 : 0.5}",
			goCode:   "var tmp0 float64\nif true {\ntmp0 = 2.5\n} else {\ntmp0 = 0.5\n}\nmap[string]interface{}{\n\"foo\": tmp0,\n}",
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

	env := environment(map[string]any{
		"a": model.StringType,
	})
	scope := env.scope()
	cases := []exprTestCase{
		{
			// TODO probably a bug in the binder. Single value objects should just be maps
			hcl2Expr: "{foo = 1.5}",
			goCode:   "map[string]interface{}{\n\"foo\": 1.5,\n}",
		},
		{
			hcl2Expr: "{\"foo\" = 1.5}",
			goCode:   "map[string]interface{}{\n\"foo\": 1.5,\n}",
		},
		{
			hcl2Expr: "{1 = 1.5}",
			goCode:   "map[string]interface{}{\n\"1\": 1.5,\n}",
		},
		{
			hcl2Expr: "{(a) = 1.5}",
			goCode:   "map[string]float64{\na: 1.5,\n}",
		},
		{
			hcl2Expr: "{(a+a) = 1.5}",
			goCode:   "map[string]float64{\na + a: 1.5,\n}",
		},
	}
	for _, c := range cases {
		testGenerateExpression(t, c.hcl2Expr, c.goCode, scope, nil)
	}
}

func TestIntrinsicConvertScopeTraversalToOutputScalar(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))
	var index bytes.Buffer

	expr := pcl.NewConvertCall(
		model.VariableReference(&model.Variable{Name: "notSecret", VariableType: model.StringType}),
		model.NewOutputType(model.StringType),
	)

	g.Fgenf(&index, "%v", expr)
	assert.Equal(t, "pulumi.String(notSecret)", index.String())
}

// Regression test for pulumi/pulumi#22256.
func TestIntrinsicConvertScopeTraversalToInputScalarNoDoubleWrap(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))
	var index bytes.Buffer

	// Resource argument Input<T> binds as union(T, Output<T>) annotated with
	// schema.InputType.
	inputType := model.NewUnionTypeAnnotated(
		[]model.Type{model.StringType, model.NewOutputType(model.StringType)},
		&schema.InputType{ElementType: schema.StringType},
	)

	expr := pcl.NewConvertCall(
		model.VariableReference(&model.Variable{Name: "bucketName", VariableType: model.StringType}),
		inputType,
	)

	g.Fgenf(&index, "%v", expr)
	assert.Equal(t, "pulumi.String(bucketName)", index.String())
}

// inputScalarType returns the union(T, Output<T>) annotated with schema.InputType that resource
// argument Input<T> binds as -- the shape whose argumentTypeName is a value constructor.
func inputScalarType() model.Type {
	return model.NewUnionTypeAnnotated(
		[]model.Type{model.StringType, model.NewOutputType(model.StringType)},
		&schema.InputType{ElementType: schema.StringType},
	)
}

// Regression test for pulumi/pulumi#22256. When the operand of a convert-to-input is
// already an Output (e.g. a traversal that reaches a nested field of a resource output,
// which lowers to an apply), it must not be wrapped in a value constructor like
// pulumi.String(...): an Output already satisfies the corresponding Input interface, and
// pulumi.String(someOutput) does not compile.
func TestIntrinsicConvertOutputToInputScalarNotWrapped(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))
	var index bytes.Buffer

	// An output-typed operand reaching a nested optional field, e.g. `res.value`
	// where value is Output<Option<string>>. This is the shape produced when a
	// program traverses a resource output object to a nested optional scalar; it
	// makes originalTo.AssignableFrom(fromType) false (optional vs non-optional
	// element), which is what previously drove the erroneous pulumi.String(...) wrap.
	from := &model.RelativeTraversalExpression{
		Source: &model.ScopeTraversalExpression{
			RootName:  "res",
			Traversal: hcl.Traversal{hcl.TraverseRoot{Name: "res"}},
			Parts:     []model.Traversable{&pcl.Resource{}},
		},
		Traversal: hcl.Traversal{hcl.TraverseAttr{Name: "value"}},
		Parts:     []model.Traversable{&model.OutputType{ElementType: model.NewOptionalType(model.StringType)}},
	}

	// Resource argument Input<T> binds as union(T, Output<T>) annotated with
	// schema.InputType.
	inputType := model.NewUnionTypeAnnotated(
		[]model.Type{model.StringType, model.NewOutputType(model.StringType)},
		&schema.InputType{ElementType: schema.StringType},
	)

	expr := pcl.NewConvertCall(from, inputType)

	g.Fgenf(&index, "%v", expr)
	assert.Equal(t, "res.Value", index.String())
}

// genInputValue (used for resource method-call args, where the binder does not insert __convert)
// wraps in pulumi.String(...) with no check for an already-Output operand.
func TestGenInputValueDoesNotWrapOutput(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))
	var buf bytes.Buffer

	// An already-Output operand, e.g. a resource output passed as a method-call arg.
	value := model.VariableReference(&model.Variable{
		Name:         "someOutput",
		VariableType: model.NewOutputType(model.StringType),
	})

	g.genInputValue(&buf, value, inputScalarType())
	assert.Equal(t, "someOutput", buf.String())
}

// genScopeTraversalExpression clears its isInput wrap only when expr.Type() is a bare
// *model.OutputType. An eventual that is not bare -- union(T, Output<T>), Option<Output<T>>,
// map(Output<T>), etc. -- slips past the guard and gets wrapped.
func TestGenScopeTraversalDoesNotWrapNonBareOutput(t *testing.T) {
	t.Parallel()

	g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))

	traversal := func(varType model.Type) string {
		var buf bytes.Buffer
		expr := &model.ScopeTraversalExpression{
			RootName:  "someOutput",
			Traversal: hcl.Traversal{hcl.TraverseRoot{Name: "someOutput"}},
			Parts:     []model.Traversable{&model.Variable{Name: "someOutput", VariableType: varType}},
		}
		g.genScopeTraversalExpression(&buf, expr, inputScalarType())
		return buf.String()
	}

	// Control: a bare Output is correctly left unwrapped by the existing guard.
	assert.Equal(t, "someOutput", traversal(model.NewOutputType(model.StringType)),
		"bare Output should not be wrapped")

	// The gap: an eventual that is not a bare *model.OutputType must also not be wrapped.
	assert.Equal(t, "someOutput",
		traversal(model.NewUnionType(model.StringType, model.NewOutputType(model.StringType))),
		"union(T, Output<T>) should not be wrapped")
}

// The alias name/type/noParent fields (genResourceOptions, gen_program.go:~1452) are wrapped in
// pulumi.String(...)/pulumi.Bool(...) with no output guard, so an alias field computed from a
// resource output produces uncompilable Go.
func TestAliasFieldDoesNotWrapOutput(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"name", "type"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			src := `
resource "sg" "infra:index:SecurityGroup" {
}
resource "sg2" "infra:index:SecurityGroup" {
    options {
        aliases = [{ ` + field + ` = sg.vpcId }]
    }
}
`
			program, diags, err := parseAndBindProgram(t, src, "alias-"+field+".pp")
			require.NoError(t, err)
			require.False(t, diags.HasErrors(), "%v", diags)

			files, gdiags, err := GenerateProgram(program)
			require.NoError(t, err)
			require.False(t, gdiags.HasErrors(), "%v", gdiags)

			// sg.VpcId is an Output; wrapping it in pulumi.String(...) does not compile.
			code := string(files["main.go"])
			assert.NotContains(t, code, "pulumi.String(sg.VpcId)",
				"alias %s must not wrap an Output in pulumi.String; got:\n%s", field, code)
		})
	}
}

func TestTupleConsExpression(t *testing.T) {
	t.Parallel()

	env := environment(map[string]any{
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
			hcl2Expr: "[1.5]",
			goCode:   "[]float64{\n1.5,\n}",
		},
		{
			hcl2Expr: "[1.5,2.5,3.5]",
			goCode:   "[]float64{\n1.5,\n2.5,\n3.5,\n}",
		},
		{
			hcl2Expr: "[1.5,\"foo\"]",
			goCode:   "[]interface{}{\n1.5,\n\"foo\",\n}",
		},
	}
	for _, c := range cases {
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
		g := newTestGenerator(t, filepath.Join("transpiled_examples", "random-pp", "random.pp"))
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
