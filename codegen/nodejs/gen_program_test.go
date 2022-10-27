package nodejs

import (
	"testing"

	"github.com/pulumi/pulumi/codegen/v3"
	"github.com/pulumi/pulumi/codegen/v3/testing/test"
)

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options-4.26": {
			Pkg:          "\"@pulumi/aws\"",
			OpAndVersion: "\"4.26.0\"",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "\"@pulumi/aws\"",
			OpAndVersion: "\"5.16.2\"",
		},
	}

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, true)
			},
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
			DependencyFile:  "package.json",
		})
}
