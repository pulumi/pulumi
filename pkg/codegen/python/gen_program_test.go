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

package python

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLengthOfOutput(t *testing.T) {
	t.Parallel()

	values := model.VariableReference(&model.Variable{
		Name:         "values",
		VariableType: model.NewOutputType(model.NewListType(model.StringType)),
	})
	expr := &model.FunctionCallExpression{
		Name: "length",
		Signature: model.StaticFunctionSignature{
			Parameters: []model.Parameter{{Name: "value", Type: values.Type()}},
			ReturnType: model.NewOutputType(model.IntType),
		},
		Args: []model.Expression{values},
	}

	g := &generator{}
	g.Formatter = format.NewFormatter(g)
	var result bytes.Buffer
	g.GenFunctionCallExpression(&result, expr)

	assert.Equal(t, "pulumi.Output.from_input(values).apply(lambda value: len(value))", result.String())
}

func TestApplyLambdaCapturesRangeValue(t *testing.T) {
	t.Parallel()

	rangeVariable := &model.Variable{Name: "range", VariableType: model.DynamicType}
	valueParameter := &model.Variable{Name: "value", VariableType: model.DynamicType}
	expr := &model.AnonymousFunctionExpression{
		Signature: model.StaticFunctionSignature{
			Parameters: []model.Parameter{{Name: "value", Type: model.DynamicType}},
			ReturnType: model.DynamicType,
		},
		Parameters: []*model.Variable{valueParameter},
		Body:       model.VariableReference(rangeVariable),
	}

	g := &generator{rangeVariable: "routes_range"}
	g.Formatter = format.NewFormatter(g)
	var result bytes.Buffer
	g.GenAnonymousFunctionExpression(&result, expr)

	assert.Equal(t, "lambda value, _routes_range=routes_range: _routes_range", result.String())
	assert.Equal(t, "routes_range", g.rangeVariable)
}

func TestComponentInputElementTypeUsesQualifiedBuiltins(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "_builtins.float", componentInputElementType(model.NumberType))
	assert.Equal(t, "list[_builtins.float]", componentInputElementType(&model.ListType{ElementType: model.NumberType}))
}

func TestFunctionInvokeBindsArgumentObjectType(t *testing.T) {
	t.Parallel()

	const source = `zones = invoke("infra:index:getZones", {})`

	program, diags := parseAndBindProgram(t, source, "bind_func_invoke_args.pp")
	contract.Ignore(diags)

	g, err := newGenerator(program)
	require.NoError(t, err)

	for _, n := range g.program.Nodes {
		if zones, ok := n.(*pcl.LocalVariable); ok && zones.Name() == "zones" {
			value := zones.Definition.Value
			funcCall, ok := value.(*model.FunctionCallExpression)
			assert.True(t, ok, "value of local variable is a function call")
			assert.Equal(t, "invoke", funcCall.Name)
			argsObject, ok := funcCall.Args[1].(*model.ObjectConsExpression)
			assert.True(t, ok, "second argument is an object expression")
			argsObjectType, ok := argsObject.Type().(*model.ObjectType)
			assert.True(t, ok, "args object has an object type")
			assert.NotEmptyf(t, argsObjectType.Annotations, "Object type should be annotated with a schema type")
			break
		}
	}
}
