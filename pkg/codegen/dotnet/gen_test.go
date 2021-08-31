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

package dotnet

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "dotnet", GeneratePackage)
}

func TestGenerateType(t *testing.T) {
	cases := []struct {
		typ      schema.Type
		expected string
	}{
		{
			&schema.InputType{
				ElementType: &schema.ArrayType{
					ElementType: &schema.InputType{
						ElementType: &schema.ArrayType{
							ElementType: &schema.InputType{
								ElementType: schema.NumberType,
							},
						},
					},
				},
			},
			"InputList<ImmutableArray<double>>",
		},
		{
			&schema.InputType{
				ElementType: &schema.MapType{
					ElementType: &schema.InputType{
						ElementType: &schema.ArrayType{
							ElementType: &schema.InputType{
								ElementType: schema.NumberType,
							},
						},
					},
				},
			},
			"InputMap<ImmutableArray<double>>",
		},
	}

	mod := &modContext{mod: "main"}
	for _, c := range cases {
		t.Run(c.typ.String(), func(t *testing.T) {
			typeString := mod.typeString(c.typ, "", true, false, false)
			assert.Equal(t, c.expected, typeString)
		})
	}
}

func TestGenerateTypeNames(t *testing.T) {
	test.TestTypeNameCodegen(t, "dotnet", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		modules, _, err := generateModuleContextMap("test", pkg)
		require.NoError(t, err)

		root, ok := modules[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t, "", false, false, false)
		}
	})
}

func TestGenerateOutputFuncsDotnet(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		assert.NoError(t, err)
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

	loadPackage := func(reader io.Reader) (*schema.Package, error) {
		var pkgSpec schema.PackageSpec
		err = json.NewDecoder(reader).Decode(&pkgSpec)
		if err != nil {
			return nil, err
		}

		pkg, err := schema.ImportSpec(pkgSpec, nil)
		if err != nil {
			return nil, err
		}
		return pkg, err
	}

	gen := func(reader io.Reader, writer io.Writer) error {
		pkg, err := loadPackage(reader)
		if err != nil {
			return nil
		}
		fun := pkg.Functions[0]
		mod := &modContext{
			pkg: pkg,
			namespaces: map[string]string{
				"azure-native":   "MadeupPackage",
				"madeup-package": "MadeupPackage",
			},
		}
		code, err := mod.genFunctionFileCode(fun)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(writer, "%s", code)
		return err
	}

	genUtilities := func(reader io.Reader, writer io.Writer) error {
		pkg, err := loadPackage(reader)
		if err != nil {
			return nil
		}
		mod := &modContext{
			pkg:           pkg,
			namespaceName: "Pulumi.MadeupPackage.Codegentest",
		}
		code, err := mod.genUtilities()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(writer, "%s", code)
		return err
	}

	dotnetDir := filepath.Join(testDir, "dotnet")

	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			inputFile := filepath.Join(testDir, fmt.Sprintf("%s.json", ex))
			expectedOutputFile := filepath.Join(dotnetDir, fmt.Sprintf("%s.cs", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)

			utilsFile := filepath.Join(testDir, "dotnet", "Utilities.cs")
			test.ValidateFileTransformer(t, inputFile, utilsFile, genUtilities)
		})
	}

	t.Run("compileGeneratedCode", func(t *testing.T) {
		t.Logf("cd %s && dotnet build", dotnetDir)
		cmd := exec.Command("dotnet", "build")
		cmd.Dir = dotnetDir
		assert.NoError(t, cmd.Run())
	})

	t.Run("testGeneratedCode", func(t *testing.T) {
		t.Logf("cd %s && dotnet test", dotnetDir)
		cmd := exec.Command("dotnet", "test")
		cmd.Dir = dotnetDir
		assert.NoError(t, cmd.Run())
	})
}
