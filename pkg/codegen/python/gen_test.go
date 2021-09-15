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
	"encoding/json"
	"fmt"
	"io"
	filesystem "io/fs"
	"io/ioutil"
	"path/filepath"
	"sort"
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
	test.TestSDKCodegen(t, "python", GeneratePackage, typeCheckGeneratedPackage)
}

// We can't type check a python program. We just check for syntax errors.
func typeCheckGeneratedPackage(t *testing.T, pwd string) {
	ex, _, err := python.CommandPath()
	require.NoError(t, err)
	cmdOptions := integration.ProgramTestOptions{}
	err = filepath.Walk(pwd, func(path string, info filesystem.FileInfo, err error) error {
		require.NoError(t, err) // an error in the walk
		if info.Mode().IsRegular() && strings.HasSuffix(info.Name(), ".py") {
			path, err = filepath.Abs(path)
			require.NoError(t, err)
			err = integration.RunCommand(t, "python syntax check",
				[]string{ex, "-m", "py_compile", path}, pwd, &cmdOptions)
			require.NoError(t, err)
		}
		return nil
	})
	require.NoError(t, err)
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

func TestGenerateOutputFuncsPython(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		require.NoError(t, err)
		return
	}

	var examples []string
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".json") {
			examples = append(examples, strings.TrimSuffix(name, ".json"))
		}
	}

	sort.Slice(examples, func(i, j int) bool { return examples[i] < examples[j] })

	gen := func(reader io.Reader, writer io.Writer) error {
		var pkgSpec schema.PackageSpec

		pkgSpec.Name = "py_tests"
		err := json.NewDecoder(reader).Decode(&pkgSpec)
		if err != nil {
			return err
		}
		pkg, err := schema.ImportSpec(pkgSpec, nil)
		if err != nil {
			return err
		}
		fun := pkg.Functions[0]

		mod := &modContext{}
		funcCode, err := mod.genFunction(fun)
		if err != nil {
			return err
		}

		_, err = writer.Write([]byte(funcCode))
		return err
	}

	outputDir, err := filepath.Abs(filepath.Join(testDir, "py_tests"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	// for every example, check that generated code did not change
	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			inputFile := filepath.Join(testDir, fmt.Sprintf("%s.json", ex))
			expectedOutputFile := filepath.Join(outputDir, fmt.Sprintf("%s.py", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)
		})
	}

	// re-generate _utilities.py to assist runtime testing
	err = ioutil.WriteFile(
		filepath.Join(outputDir, "_utilities.py"),
		genUtilitiesFile("gen_test.go"),
		0600)

	require.NoError(t, err)

	// run unit tests against the generated code
	t.Run("testGeneratedCode", func(t *testing.T) {
		venvDir := filepath.Join(outputDir, "venv")

		t.Logf("cd %s && python3 -m venv venv", outputDir)
		t.Logf("cd %s && ./venv/bin/python -m pip install -r requirements.txt", outputDir)
		err := python.InstallDependencies(outputDir, venvDir, false /*showOutput*/)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..",
			"sdk", "python", "env", "src"))
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		t.Logf("cd %s && ./venv/bin/python -m pip install -e %s", outputDir, sdkDir)
		cmd := python.VirtualEnvCommand(venvDir, "python", "-m", "pip", "install", "-e", sdkDir)
		cmd.Dir = outputDir
		err = cmd.Run()
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		t.Logf("cd %s && ./venv/bin/python -m pip install -e .", outputDir)
		cmd = python.VirtualEnvCommand(venvDir, "python", "-m", "pip", "install", "-e", ".")
		cmd.Dir = outputDir
		err = cmd.Run()
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		t.Logf("cd %s && ./venv/bin/pytest", outputDir)
		cmd = python.VirtualEnvCommand(venvDir, "pytest", ".")
		cmd.Dir = outputDir
		err = cmd.Run()
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	})
}
