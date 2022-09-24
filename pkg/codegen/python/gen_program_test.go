package python

import (
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

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options": {
			Pkg:          "pulumi-aws",
			OpAndVersion: "==4.38.0",
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
					Directory:   "aws-resource-options",
					Description: "Resource Options",
					MockPluginVersions: map[string]string{
						"aws": "4.38.0",
					},
				},
			},

			IsGenProject:    true,
			GenProject:      GenerateProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "requirements.txt",
		},
	)
}
