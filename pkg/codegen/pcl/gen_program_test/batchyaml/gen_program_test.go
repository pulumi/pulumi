package batchyaml

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples
func TestGenerateProgram(t *testing.T) {
	t.Parallel()
	err := os.Chdir("../../../pcl") // chdir into codegen/python
	assert.NoError(t, err)

	os.Setenv("PULUMI_ACCEPT", "true")
	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "pcl_json",
			Extension:  "json",
			OutputFile: "main.pcl.json",
			GenProgram: pcl.GenerateJSONProgram,
			TestCases:  test.PulumiPulumiYAMLProgramTests,
		})
}
