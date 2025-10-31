package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

func GeneratePythonProgramTest(t *testing.T, genProgram GenProgram, genProject GenProject) {
	test.GeneratePythonProgramTest(t, genProgram, genProject)
}

func GeneratePythonBatchTest(t *testing.T, rootDir string, genProgram GenProgram, testCases []ProgramTest) {
	test.GeneratePythonBatchTest(t, rootDir, genProgram, testCases)
}

func GeneratePythonYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	test.GeneratePythonYAMLBatchTest(t, rootDir, genProgram)
}

// Checks generated code for syntax errors with `python -m compile`.
func CompilePython(t *testing.T, codeDir string) {
	test.CompilePython(t, codeDir)
}

