package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

func GenerateGoProgramTest(t *testing.T, rootDir string, genProgram GenProgram, genProject GenProject) {
	test.GenerateGoProgramTest(t, rootDir, genProgram, genProject)
}

func GenerateGoBatchTest(t *testing.T, rootDir string, genProgram GenProgram, testCases []ProgramTest) {
	test.GenerateGoBatchTest(t, rootDir, genProgram, testCases)
}

func GenerateGoYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	test.GenerateGoYAMLBatchTest(t, rootDir, genProgram)
}

