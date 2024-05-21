package python

import (
	"context"
	filesystem "io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
)

func Check(t *testing.T, path string, _ codegen.StringSet) {
	pyCompileCheck(t, filepath.Dir(path))
}

// Checks generated code for syntax errors with `python -m compile`.
func pyCompileCheck(t *testing.T, codeDir string) {
	pythonFiles := []string{}
	err := filepath.Walk(codeDir, func(path string, info filesystem.FileInfo, err error) error {
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

	// TODO: julienp
	// Should this be cleaned up; test.RunCommand wants the path to the executable, we get it by
	// creating a command and then getting the path from it.
	tc, err := toolchain.ResolveToolchain(
		codeDir,
		toolchain.PythonOptions{
			// TODO: should this take venv?
			PackageManager: toolchain.PackageManagerPip,
		})
	require.NoError(t, err)
	args := append([]string{"-m", "py_compile"}, pythonFiles...)
	cmd, err := tc.Command(context.Background(), args...)
	require.NoError(t, err)
	test.RunCommand(t, "python syntax check", codeDir, cmd.Path, args...)
}

func GenerateProgramBatchTest(t *testing.T, testCases []test.ProgramTest) {
	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      Check,
			GenProgram: GenerateProgram,
			TestCases:  testCases,
		})
}
