// Copyright 2016-2021, Pulumi Corporation.
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
	"fmt"
	filesystem "io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

var pathTests = []struct {
	input    string
	expected string
}{
	{".", "."},
	{"", "."},
	{"../", ".."},
	{"../..", "..."},
	{"../../..", "...."},
	{"something", ".something"},
	{"../parent", "..parent"},
	{"../../module", "...module"},
}

func TestRelPathToRelImport(t *testing.T) {
	t.Parallel()

	for _, tt := range pathTests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result := relPathToRelImport(tt.input)
			if result != tt.expected {
				t.Errorf("expected \"%s\"; got \"%s\"", tt.expected, result)
			}
		})
	}
}

func TestGeneratePackage(t *testing.T) {
	t.Parallel()

	if !test.NoSDKCodegenChecks() {
		// To speed up these tests, we will generate one common
		// virtual environment for all of them to run in, rather than
		// having one per test.
		err := buildVirtualEnv()
		if err != nil {
			t.Error(err)
			return
		}
	}

	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "python",
		GenPackage: GeneratePackage,
		Checks: map[string]test.CodegenCheck{
			"python/py_compile": pyCompileCheck,
			"python/test":       pyTestCheck,
		},
		TestCases: test.PulumiPulumiSDKTests,
	})
}

func absTestsPath() (string, error) {
	hereDir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	return hereDir, nil
}

func virtualEnvPath() (string, error) {
	hereDir, err := absTestsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(hereDir, "venv"), nil
}

// To serialize shared `venv` operations; without the lock running
// tests with `-parallel` causes sproadic failure.
var venvMutex = &sync.Mutex{}

func buildVirtualEnv() error {
	hereDir, err := absTestsPath()
	if err != nil {
		return err
	}
	venvDir, err := virtualEnvPath()
	if err != nil {
		return err
	}

	gotVenv, err := test.PathExists(venvDir)
	if err != nil {
		return err
	}

	if gotVenv {
		err := os.RemoveAll(venvDir)
		if err != nil {
			return err
		}
	}

	err = python.InstallDependencies(hereDir, venvDir, false /*showOutput*/)
	if err != nil {
		return err
	}

	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "python", "env", "src"))
	if err != nil {
		return err
	}

	gotSdk, err := test.PathExists(sdkDir)
	if err != nil {
		return err
	}

	if !gotSdk {
		return fmt.Errorf("This test requires Python SDK to be built; please `cd sdk/python && make ensure build install`")
	}

	return nil
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

	ex, _, err := python.CommandPath()
	require.NoError(t, err)
	args := append([]string{"-m", "py_compile"}, pythonFiles...)
	test.RunCommand(t, "python syntax check", codeDir, ex, args...)
}

func pyTestCheck(t *testing.T, codeDir string) {
	venvDir, err := virtualEnvPath()
	if err != nil {
		t.Error(err)
		return
	}

	cmd := func(name string, args ...string) error {
		t.Logf("cd %s && %s %s", codeDir, name, strings.Join(args, " "))
		cmd := python.VirtualEnvCommand(venvDir, name, args...)
		cmd.Dir = codeDir
		return cmd.Run()
	}

	installPackage := func() error {
		venvMutex.Lock()
		defer venvMutex.Unlock()
		return cmd("python", "-m", "pip", "install", "-e", ".")
	}

	if err = installPackage(); err != nil {
		t.Error(err)
		return
	}

	if err = cmd("pytest", "."); err != nil {
		exitError, isExitError := err.(*exec.ExitError)
		if isExitError && exitError.ExitCode() == 5 {
			t.Logf("Could not find any pytest tests in %s", codeDir)
		} else {
			t.Error(err)
		}
		return
	}
}

func TestGenerateTypeNames(t *testing.T) {
	t.Parallel()

	test.TestTypeNameCodegen(t, "python", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		// Decode python-specific info
		err := pkg.ImportLanguages(map[string]schema.Language{"python": Importer})
		require.NoError(t, err)

		info, _ := pkg.Language["python"].(PackageInfo)

		modules, err := generateModuleContextMap("test", pkg, info, nil)
		require.NoError(t, err)

		root, ok := modules[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t, false, false)
		}
	})
}
