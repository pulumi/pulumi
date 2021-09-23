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
	filesystem "io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
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
	for _, tt := range pathTests {
		t.Run(tt.input, func(t *testing.T) {
			result := relPathToRelImport(tt.input)
			if result != tt.expected {
				t.Errorf("expected \"%s\"; got \"%s\"", tt.expected, result)
			}
		})
	}
}

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language:   "python",
		GenPackage: GeneratePackage,
		Checks: map[string]test.CodegenCheck{
			"python/py_compile": pyCompileCheck,
			"python/test":       pyTestCheck,
		},
	})
}

// Checks generated code for syntax errors with `python -m compile`.
func pyCompileCheck(t *testing.T, codeDir string) {
	ex, _, err := python.CommandPath()
	require.NoError(t, err)
	cmdOptions := integration.ProgramTestOptions{}
	err = filepath.Walk(codeDir, func(path string, info filesystem.FileInfo, err error) error {
		require.NoError(t, err) // an error in the walk

		if info.Mode().IsDir() && info.Name() == "venv" {
			return filepath.SkipDir
		}

		if info.Mode().IsRegular() && strings.HasSuffix(info.Name(), ".py") {
			path, err = filepath.Abs(path)
			require.NoError(t, err)
			err = integration.RunCommand(t, "python syntax check",
				[]string{ex, "-m", "py_compile", path}, codeDir, &cmdOptions)
			require.NoError(t, err)
		}
		return nil
	})
	require.NoError(t, err)
}

func pyTestCheck(t *testing.T, codeDir string) {
	venvDir, err := filepath.Abs(filepath.Join(codeDir, "venv"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	t.Logf("cd %s && python3 -m venv venv", codeDir)
	t.Logf("cd %s && ./venv/bin/python -m pip install -r requirements.txt", codeDir)
	err = python.InstallDependencies(codeDir, venvDir, true /*showOutput*/)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "python", "env", "src"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	gotSdk, err := test.PathExists(sdkDir)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !gotSdk {
		t.Errorf("This test requires Python SDK to be built; please `cd sdk/python && make ensure build install`")
		t.FailNow()
	}

	cmd := func(name string, args ...string) {
		t.Logf("cd %s && ./venv/bin/%s %s", codeDir, name, strings.Join(args, " "))
		cmd := python.VirtualEnvCommand(venvDir, name, args...)
		cmd.Dir = codeDir
		err = cmd.Run()
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}

	cmd("python", "-m", "pip", "install", "-e", sdkDir)
	cmd("python", "-m", "pip", "install", "-e", ".")
	cmd("pytest", ".")
}

func TestGenerateTypeNames(t *testing.T) {
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
