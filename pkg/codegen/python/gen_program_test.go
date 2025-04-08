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

package python

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

func TestFunctionInvokeBindsArgumentObjectType(t *testing.T) {
	t.Parallel()

	const source = `zones = invoke("aws:index:getAvailabilityZones", {})`

	program, diags := parseAndBindProgram(t, source, "bind_func_invoke_args.pp")
	contract.Ignore(diags)

	g, err := newGenerator(program)
	assert.NoError(t, err)

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

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	test.GeneratePythonProgramTest(
		t,
		GenerateProgram,
		func(
			directory string, project workspace.Project,
			program *pcl.Program, localDependencies map[string]string,
		) error {
			return GenerateProject(directory, project, program, localDependencies, "")
		},
	)
}

func TestGenFunctionCallConvertToOutput(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	gen := &generator{}
	gen.Formatter = format.NewFormatter(gen)

	gen.GenFunctionCallExpression(buffer, &model.FunctionCallExpression{
		Name: "__convert",
		Signature: model.StaticFunctionSignature{
			Parameters: []model.Parameter{
				{
					Name: "value",
					Type: model.InputType(model.NumberType),
				},
			},
			ReturnType: &model.OutputType{
				ElementType: model.NumberType,
			},
		},
		Args: []model.Expression{
			model.VariableReference(
				&model.Variable{
					Name:         "some_number_input",
					VariableType: model.InputType(model.NumberType),
				},
			),
		},
	})

	assert.Equal(t, "pulumi.Output.from_input(some_number_input)", buffer.String())
}

func TestGenFunctionCallOutputsDontAddConvertToOutput(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	gen := &generator{}
	gen.Formatter = format.NewFormatter(gen)

	gen.GenFunctionCallExpression(buffer, &model.FunctionCallExpression{
		Name: "__convert",
		Signature: model.StaticFunctionSignature{
			Parameters: []model.Parameter{
				{
					Name: "value",
					Type: model.InputType(model.NumberType),
				},
			},
			ReturnType: &model.OutputType{
				ElementType: model.NumberType,
			},
		},
		Args: []model.Expression{
			model.VariableReference(
				&model.Variable{
					Name:         "some_number_input",
					VariableType: &model.OutputType{ElementType: model.NumberType},
				},
			),
		},
	})

	assert.Equal(t, "some_number_input", buffer.String())
}
