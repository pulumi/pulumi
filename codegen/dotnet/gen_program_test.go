package dotnet

import (
	"testing"

	"github.com/pulumi/pulumi/codegen/v3"
	"github.com/pulumi/pulumi/codegen/v3/testing/test"
)

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options-4.26": {
			Pkg:          "<PackageReference Include=\"Pulumi.Aws\"",
			OpAndVersion: "Version=\"4.26.0\"",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "<PackageReference Include=\"Pulumi.Aws\"",
			OpAndVersion: "Version=\"5.16.2\"",
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
			DependencyFile:  "test.csproj",
		},
	)
}
