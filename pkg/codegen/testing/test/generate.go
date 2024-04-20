package test

import (
	"testing"
)

// GenerateProgramBatchTest returns a batch generator for the given language.
func GenerateProgramBatchTest(language string) func(*testing.T, GenProgram, []ProgramTest) {
	switch language {
	case "nodejs":
		return generateNodeBatchTest
	case "dotnet":
		return generateDotnetBatchTest
	case "go":
		return generateGoBatchTest
	case "python":
		return generatePythonBatchTest
	default:
		panic("unrecognized language " + language)
	}
}
