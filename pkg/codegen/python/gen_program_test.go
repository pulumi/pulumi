package python

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestGenerateProgram(t *testing.T) {
	t.Parallel()

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      Check,
			GenProgram: GenerateProgram,
			TestCases:  test.PulumiPulumiProgramTests,
		})
}

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

			IsGenProject:    true,
			GenProject:      GenerateProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "requirements.txt",
		},
	)
}
