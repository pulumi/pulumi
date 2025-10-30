package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

func GenerateDotnetProgramTest(t *testing.T, genProgram GenProgram, genProject GenProject) {
	test.GenerateDotnetProgramTest(t, genProgram, genProject)
}

func GenerateDotnetBatchTest(t *testing.T, rootDir string, genProgram GenProgram, testCases []ProgramTest) {
	test.GenerateDotnetBatchTest(t, rootDir, genProgram, testCases)
}

func GenerateDotnetYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	test.GenerateDotnetYAMLBatchTest(t, rootDir, genProgram)
}

