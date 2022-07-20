package dotnet

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

func TestGenerateProgram(t *testing.T) {
	t.Parallel()

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, "../../../../../../../sdk/dotnet/Pulumi")
			},
			GenProgram: GenerateProgram,
			TestCases:  test.PulumiPulumiProgramTests,
		},
	)
}

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options": {
			Pkg:          "<PackageReference Include=\"Pulumi.Aws\"",
			OpAndVersion: "Version=\"4.38.0\"",
		},
	}

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, "../../../../../../../sdk/dotnet/Pulumi")
			},
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
			DependencyFile:  "test.csproj",
		},
	)
}
