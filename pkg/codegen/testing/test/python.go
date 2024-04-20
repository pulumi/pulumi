package test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

func CheckPython(t *testing.T, path string, _ codegen.StringSet) {
	pyCompileCheck(t, filepath.Dir(path))
}

// Checks generated code for syntax errors with `python -m compile`.
func pyCompileCheck(t *testing.T, codeDir string) {
	pythonFiles := []string{}
	err := filepath.Walk(codeDir, func(path string, info fs.FileInfo, err error) error {
		require.NoError(t, err) // an error in the walk

		if info.Mode().IsDir() && info.Name() == "venv" {
			return filepath.SkipDir
		}

		if info.Mode().IsRegular() && strings.HasSuffix(info.Name(), ".py") {
			path, err = filepath.Abs(path)
			require.NoError(t, err)

			pythonFiles = append(pythonFiles, path)
		}
		return nil
	})
	require.NoError(t, err)

	ex, _, err := python.CommandPath()
	require.NoError(t, err)
	args := append([]string{"-m", "py_compile"}, pythonFiles...)
	RunCommand(t, "python syntax check", codeDir, ex, args...)
}

func generatePythonBatchTest(t *testing.T, generator GenProgram, testCases []ProgramTest) {
	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      CheckPython,
			GenProgram: generator,
			TestCases:  testCases,
		})
}
