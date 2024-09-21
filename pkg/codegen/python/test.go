// Copyright 2022-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	// Find the path to global python
	tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
		Toolchain: toolchain.Pip,
	})
	require.NoError(t, err)
	info, err := tc.About(context.Background())
	require.NoError(t, err)
	pythonCmdPath := info.Executable
	// Run `python -m py_compile` on all python files
	args := append([]string{"-m", "py_compile"}, pythonFiles...)
	test.RunCommand(t, "python syntax check", codeDir, pythonCmdPath, args...)
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
