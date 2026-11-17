// Copyright 2020, Pulumi Corporation.
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

//nolint:lll
package nodejs

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// For better CI test to job distribution, we split the test cases into three tests.

var genPkgBatchSize = len(test.PulumiPulumiSDKTests) / 3

func TestGeneratePackageOne(t *testing.T) {
	t.Parallel()

	testGeneratePackageBatch(t, test.PulumiPulumiSDKTests[0:genPkgBatchSize])
}

func TestGeneratePackageTwo(t *testing.T) {
	t.Parallel()

	testGeneratePackageBatch(t, test.PulumiPulumiSDKTests[genPkgBatchSize:2*genPkgBatchSize])
}

func TestGeneratePackageThree(t *testing.T) {
	t.Parallel()

	testGeneratePackageBatch(t, test.PulumiPulumiSDKTests[2*genPkgBatchSize:])
}

func testGeneratePackageBatch(t *testing.T, testCases []*test.SDKTest) {
	test.TestSDKCodegen(t, &test.SDKCodegenOptions{
		Language: "nodejs",
		GenPackage: func(s string, p *schema.Package, m map[string][]byte, l schema.ReferenceLoader) (map[string][]byte, error) {
			return GeneratePackage(s, p, m, nil, false, l)
		},
		Checks: map[string]test.CodegenCheck{
			"nodejs/compile": func(t *testing.T, pwd string) {
				test.TypeCheckNodeJSPackage(t, pwd, true)
			},
			"nodejs/test": testGeneratedPackage,
		},
		TestCases: testCases,
	})
}

// Runs unit tests against the generated code.
func testGeneratedPackage(t *testing.T, pwd string) {
	_, mochaErr := os.Stat(filepath.Join(pwd, "node_modules", ".bin", "mocha"))
	hasMocha := mochaErr == nil

	// We are attempting to ensure that we don't write tests that are not run. The `nodejs-extras`
	// folder exists to mixin tests of the form `*.spec.ts`. We assume that if this folder is
	// present and contains `*.spec.ts` files, we want to run those tests.
	foundTests := false
	findTests := func(path string, _ os.DirEntry, _ error) error {
		if strings.HasSuffix(path, ".spec.ts") {
			foundTests = true
		}
		return nil
	}
	mixinFolder := filepath.Join(filepath.Dir(pwd), "nodejs-extras")
	if err := filepath.WalkDir(mixinFolder, findTests); !hasMocha && !os.IsNotExist(err) && foundTests {
		t.Errorf("%s has at least one nodejs-extras/**/*.spec.ts file , but does not have mocha as a dependency."+
			" Tests were not run. Please add mocha as a dependency in the schema or remove the *.spec.ts files.",
			pwd)
	}

	if hasMocha {
		// If mocha is a dev dependency but no test files exist, this will fail.
		test.RunCommand(t, "mocha", pwd,
			filepath.Join(pwd, "node_modules", ".bin", "mocha"),
			"--timeout", "120000",
			"--require", "ts-node/register",
			"tests/**/*.spec.ts")
	} else {
		t.Logf("No mocha tests found for %s", pwd)
	}
}

func TestGenerateTypeNames(t *testing.T) {
	t.Parallel()

	test.TestTypeNameCodegen(t, "nodejs", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		modules, info, err := generateModuleContextMap("test", pkg, nil)
		require.NoError(t, err)

		pkg.Language["nodejs"] = info

		root, ok := modules[""]
		require.True(t, ok)

		// Parallel tests will use the TypeNameGeneratorFunc
		// from multiple goroutines, but root.typeString is
		// not safe. Mutex is needed to avoid panics on
		// concurrent map write.
		//
		// Note this problem is test-only since prod code
		// works on a single goroutine.

		var mutex sync.Mutex
		return func(t schema.Type) string {
			mutex.Lock()
			defer mutex.Unlock()
			return root.typeString(t, false, nil)
		}
	}, filepath.FromSlash("../testing/test/testdata/"))
}

func TestGenerateSelfReferencingResource(t *testing.T) {
	t.Parallel()

	for _, shape := range []string{"direct", "array", "map", "union"} {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()

			properties := map[string]schema.PropertySpec{}
			for _, name := range []string{"Node", "Other"} {
				ref := schema.TypeSpec{Ref: "#/resources/example:index:" + name}
				typ := ref
				switch shape {
				case "array":
					typ = schema.TypeSpec{Type: "array", Items: &ref}
				case "map":
					typ = schema.TypeSpec{Type: "object", AdditionalProperties: &ref}
				case "union":
					typ = schema.TypeSpec{OneOf: []schema.TypeSpec{typ, {Type: "string"}}}
				}
				properties[strings.ToLower(name)] = schema.PropertySpec{TypeSpec: typ}
			}
			pkg, err := schema.ImportSpec(schema.PackageSpec{
				Name: "example",
				Resources: map[string]schema.ResourceSpec{
					"example:index:Node":  {InputProperties: properties},
					"example:index:Other": {},
				},
			}, nil, schema.NewNullLoader(), schema.ValidationOptions{})
			require.NoError(t, err)

			files, err := GeneratePackage("test", pkg, nil, nil, false, nil)
			require.NoError(t, err)
			require.Contains(t, files, "node.ts")
			source := string(files["node.ts"])
			require.Contains(t, source, "export class Node extends pulumi.CustomResource")
			require.Contains(t, source, `import {Other} from "./index";`)
			for line := range strings.SplitSeq(source, "\n") {
				if strings.HasPrefix(line, "import ") {
					require.NotContains(t, line, "Node")
				}
			}
		})
	}
}

func TestPascalCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "hi",
			expected: "Hi",
		},
		{
			input:    "NothingChanges",
			expected: "NothingChanges",
		},
		{
			input:    "everything-changed",
			expected: "EverythingChanged",
		},
	}
	for _, tt := range tests {
		result := pascal(tt.input)
		require.Equal(t, tt.expected, result)
	}
}

func Test_isStringType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    schema.Type
		expected bool
	}{
		{"string", schema.StringType, true},
		{"int", schema.IntType, false},
		{"Input[string]", &schema.InputType{ElementType: schema.StringType}, true},
		{"Input[int]", &schema.InputType{ElementType: schema.IntType}, false},
		{"StrictStringEnum", &schema.EnumType{ElementType: schema.StringType}, true},
		{"StrictIntEnum", &schema.EnumType{ElementType: schema.IntType}, false},
		{"RelaxedStringEnum", &schema.UnionType{
			ElementTypes: []schema.Type{&schema.EnumType{ElementType: schema.StringType}, schema.StringType},
		}, true},
		{"RelaxedIntEnum", &schema.UnionType{
			ElementTypes: []schema.Type{&schema.EnumType{ElementType: schema.IntType}, schema.IntType},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isStringType(tt.input); got != tt.expected {
				t.Errorf("isStringType() = %v, want %v", got, tt.expected)
			}
		})
	}
}
