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

package test

import (
	"context"
	filesystem "io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
)

func GeneratePythonProgramTest(
	t *testing.T,
	genProgram GenProgram,
	genProject GenProject,
) {
	expectedVersion := map[string]PkgVersionInfo{
		"aws-resource-options-4.3.8": {
			Pkg:          "pulumi-aws",
			OpAndVersion: "==4.26.0",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "pulumi-aws",
			OpAndVersion: "==5.16.2",
		},
	}

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      checkPython,
			GenProgram: genProgram,
			TestCases: []ProgramTest{
				{
					Directory:   "aws-resource-options-4.26",
					Description: "Resource Options",
				},
				{
					Directory:   "aws-resource-options-5.16.2",
					Description: "Resource Options",
				},
			},

			IsGenProject:    true,
			GenProject:      genProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "requirements.txt",
		},
	)
}

func GeneratePythonBatchTest(t *testing.T, rootDir string, genProgram GenProgram, testCases []ProgramTest) {
	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      checkPython,
			GenProgram: genProgram,
			TestCases:  testCases,
		})
}

func GeneratePythonYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	err := os.Chdir(filepath.Join(rootDir, "pkg", "codegen", "python"))
	require.NoError(t, err)

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "python",
			Extension:  "py",
			OutputFile: "__main__.py",
			Check:      checkPython,
			GenProgram: genProgram,
			TestCases:  PulumiPulumiYAMLProgramTests,
		})
}

func checkPython(t *testing.T, path string, _ codegen.StringSet) {
	CompilePython(t, filepath.Dir(path))
}

// Checks generated code for syntax errors with `python -m compile`.
func CompilePython(t *testing.T, codeDir string) {
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
	RunCommand(t, "python syntax check", codeDir, pythonCmdPath, args...)
}
