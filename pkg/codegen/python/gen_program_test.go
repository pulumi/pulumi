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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
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

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options-4.3.8": {
			Pkg:          "pulumi-aws",
			OpAndVersion: "==4.26.0",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "pulumi-aws",
			OpAndVersion: "==5.16.2",
		},
	}

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      Check,
			GenProgram: GenerateProgram,
			TestCases: []test.ProgramTest{
				{
					Directory:   "aws-resource-options-4.26",
					Description: "Resource Options",
				},
				{
					Directory:   "aws-resource-options-5.16.2",
					Description: "Resource Options",
				},
			},

			IsGenProject: true,
			GenProject: func(
				directory string, project workspace.Project,
				program *pcl.Program, localDependencies map[string]string,
			) error {
				return GenerateProject(directory, project, program, localDependencies, "")
			},
			ExpectedVersion: expectedVersion,
			DependencyFile:  "requirements.txt",
		},
	)
}
