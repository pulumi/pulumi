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
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

func TestFunctionInvokeBindsArgumentObjectType(t *testing.T) {
	t.Parallel()

	const source = `zones = invoke("aws:index:getAvailabilityZones", {})`

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

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	test.GeneratePythonProgramTest(
		t,
		GenerateProgram,
		func(
			directory string, project workspace.Project,
			program *pcl.Program, localDependencies map[string]string,
		) error {
			return GenerateProject(directory, project, program, localDependencies, "", "")
		},
	)
}

func TestGenerateProjectWithTypechecker(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		typechecker         string
		expectedRequirement string
	}{
		{
			name:                "mypy typechecker",
			typechecker:         "mypy",
			expectedRequirement: "mypy>=1.0.0",
		},
		{
			name:                "pyright typechecker",
			typechecker:         "pyright",
			expectedRequirement: "pyright>=1.1.0",
		},
		{
			name:                "no typechecker",
			typechecker:         "",
			expectedRequirement: "",
		},
		{
			name:                "unknown typechecker",
			typechecker:         "custom-checker",
			expectedRequirement: "custom-checker",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create a minimal program
			program, diags := parseAndBindProgram(t, ``, "test.pp")
			assert.False(t, diags.HasErrors())

			// Create a temporary directory
			tempDir := t.TempDir()

			// Create a basic project
			project := workspace.Project{
				Name: "test-project",
				Runtime: workspace.NewProjectRuntimeInfo("python", map[string]interface{}{
					"toolchain": "pip",
				}),
			}

			// Call GenerateProject with the typechecker
			err := GenerateProject(tempDir, project, program, map[string]string{}, tc.typechecker, "pip")
			require.NoError(t, err)

			// Read the generated requirements.txt
			requirementsPath := tempDir + "/requirements.txt"
			requirementsContent, err := os.ReadFile(requirementsPath)
			require.NoError(t, err)

			requirementsStr := string(requirementsContent)

			// Check that pulumi is always included
			assert.Contains(t, requirementsStr, "pulumi>=3.0.0,<4.0.0")

			// Check typechecker requirement
			if tc.expectedRequirement != "" {
				assert.Contains(t, requirementsStr, tc.expectedRequirement)
			} else {
				// If no typechecker, ensure no typechecker packages are included
				assert.NotContains(t, requirementsStr, "mypy")
				assert.NotContains(t, requirementsStr, "pyright")
			}
		})
	}
}
