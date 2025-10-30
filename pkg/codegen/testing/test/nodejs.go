package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

func GenerateNodeJSProgramTest(t *testing.T, genProgram GenProgram, genProject GenProject) {
	test.GenerateNodeJSProgramTest(t, genProgram, genProject)
}

func GenerateNodeJSBatchTest(t *testing.T, rootDir string, genProgram GenProgram, testCases []ProgramTest) {
	test.GenerateNodeJSBatchTest(t, rootDir, genProgram, testCases)
}

func GenerateNodeJSYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	test.GenerateNodeJSYAMLBatchTest(t, rootDir, genProgram)
}

func TypeCheckNodeJSPackage(t *testing.T, pwd string, linkLocal bool) {
	test.TypeCheckNodeJSPackage(t, pwd, linkLocal)
}

