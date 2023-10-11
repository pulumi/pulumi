package batchyaml

import (
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	codegenGo "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples, as it requires a different SDK path in Check
func TestGenerateProgram(t *testing.T) {
	t.Parallel()
	err := os.Chdir("../../../go") // chdir into codegen/go
	assert.NoError(t, err)

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				codegenGo.Check(t, path, dependencies, "../../../../../../../../sdk")
			},
			GenProgram: func(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
				// Prevent tests from interfering with each other
				return codegenGo.GenerateProgramWithOptions(program,
					codegenGo.GenerateProgramOptions{ExternalCache: codegenGo.NewCache()})
			},
			TestCases: test.PulumiPulumiYAMLProgramTests,
		})
}
