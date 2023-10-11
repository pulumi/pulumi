package batchyaml

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	codegenPy "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples
func TestGenerateProgram(t *testing.T) {
	t.Parallel()
	err := os.Chdir("../../../python") // chdir into codegen/python
	assert.NoError(t, err)

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      codegenPy.Check,
			GenProgram: codegenPy.GenerateProgram,
			TestCases:  test.PulumiPulumiYAMLProgramTests,
		})
}
