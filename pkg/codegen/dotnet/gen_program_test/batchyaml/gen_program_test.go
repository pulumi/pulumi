package batchyaml

import (
	"os"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"

	codegenDotnet "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples, as it requires a different SDK path in Check
func TestGenerateProgram(t *testing.T) {
	t.Parallel()
	err := os.Chdir("../../../dotnet") // chdir into codegen/dotnet
	assert.Nil(t, err)

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "dotnet",
			Extension:  "cs",
			OutputFile: "Program.cs",
			Check: func(t *testing.T, path string, dependencies mapset.Set[string]) {
				codegenDotnet.Check(t, path, dependencies, "")
			},
			GenProgram: codegenDotnet.GenerateProgram,
			TestCases:  test.PulumiPulumiYAMLProgramTests,
		},
	)
}
