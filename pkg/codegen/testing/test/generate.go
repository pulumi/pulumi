package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

const TestDotnet = test.TestDotnet

const TestGo = test.TestGo

const TestNodeJS = test.TestNodeJS

const TestPython = test.TestPython

// GenerateProgramBatchTest returns a batch generator for the given language.
func GenerateProgramBatchTest(language string) func(*testing.T, string, GenProgram, []ProgramTest) {
	return test.GenerateProgramBatchTest(language)
}

