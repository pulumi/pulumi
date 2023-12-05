package batchyaml

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	codegenNode "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples
func TestGenerateProgram(t *testing.T) {
	t.Parallel()
	err := os.Chdir("../../../nodejs") // chdir into codegen/nodejs
	assert.NoError(t, err)

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				codegenNode.Check(t, path, dependencies, true)
			},
			GenProgram: codegenNode.GenerateProgram,
			TestCases:  test.PulumiPulumiYAMLProgramTests,
		})
}
