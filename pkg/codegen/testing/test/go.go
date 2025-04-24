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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func GenerateGoProgramTest(
	t *testing.T,
	rootDir string,
	genProgram GenProgram,
	genProject GenProject,
) {
	expectedVersion := map[string]PkgVersionInfo{
		"aws-resource-options-4.26": {
			Pkg:          "github.com/pulumi/pulumi-aws/sdk/v4",
			OpAndVersion: "v4.26.0",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "github.com/pulumi/pulumi-aws/sdk/v5",
			OpAndVersion: "v5.16.2",
		},
		"modpath": {
			Pkg:          "git.example.org/thirdparty/sdk",
			OpAndVersion: "v0.1.0",
		},
	}

	sdkDir, err := filepath.Abs(filepath.Join(rootDir, "sdk"))
	require.NoError(t, err)

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				checkGo(t, path, dependencies, sdkDir)
			},
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
				{
					Directory:   "modpath",
					Description: "Check that modpath is respected",
					MockPluginVersions: map[string]string{
						"other": "0.1.0",
					},
					// We don't compile because the test relies on the `other` package,
					// which does not exist.
					SkipCompile: codegen.NewStringSet("go"),
				},
			},

			IsGenProject:    true,
			GenProject:      genProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "go.mod",
		})
}

func GenerateGoBatchTest(
	t *testing.T,
	rootDir string,
	genProgram GenProgram,
	testCases []ProgramTest,
) {
	sdkDir, err := filepath.Abs(filepath.Join(rootDir, "sdk"))
	require.NoError(t, err)

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				checkGo(t, path, dependencies, sdkDir)
			},
			GenProgram: genProgram,
			TestCases:  testCases,
		})
}

func GenerateGoYAMLBatchTest(t *testing.T, rootDir string, genProgram GenProgram) {
	sdkDir, err := filepath.Abs(filepath.Join(rootDir, "sdk"))
	require.NoError(t, err)

	err = os.Chdir(filepath.Join(rootDir, "pkg", "codegen", "go"))
	require.NoError(t, err)

	TestProgramCodegen(t,
		ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				checkGo(t, path, dependencies, sdkDir)
			},
			GenProgram: genProgram,
			TestCases:  PulumiPulumiYAMLProgramTests,
		})
}

func checkGo(t *testing.T, path string, deps codegen.StringSet, pulumiSDKPath string) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	// We remove go.mod to ensure tests are reproducible.
	goMod := filepath.Join(dir, "go.mod")
	if err = os.Remove(goMod); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	err = integration.RunCommand(t, "generate go.mod",
		[]string{ex, "mod", "init", "main"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	err = integration.RunCommand(t, "go tidy",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	if pulumiSDKPath != "" {
		err = integration.RunCommand(t, "point towards local Go SDK",
			[]string{
				ex, "mod", "edit",
				fmt.Sprintf("--replace=%s=%s",
					"github.com/pulumi/pulumi/sdk/v3",
					pulumiSDKPath),
			},
			dir, &integration.ProgramTestOptions{})
		require.NoError(t, err)
	}
	typeCheckGo(t, path, deps, pulumiSDKPath)
}

func typeCheckGo(t *testing.T, path string, deps codegen.StringSet, pulumiSDKPath string) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	err = integration.RunCommand(t, "go tidy after replace",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)

	err = integration.RunCommand(t, "test build", []string{ex, "build", "-v", "all"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	os.Remove(filepath.Join(dir, "main"))
	assert.NoError(t, err)
}
