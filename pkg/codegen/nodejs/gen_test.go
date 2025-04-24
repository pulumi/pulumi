// Copyright 2020-2024, Pulumi Corporation.
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
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
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
	// Some tests have do not have mocha as a dependency.
	hasMocha := false
	for _, c := range getYarnCommands(t, pwd) {
		if c == "mocha" {
			hasMocha = true
			break
		}
	}

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
			"yarn", "run", "mocha",
			"--require", "ts-node/register",
			"tests/**/*.spec.ts")
	} else {
		t.Logf("No mocha tests found for %s", pwd)
	}
}

// Get the commands runnable with yarn run
func getYarnCommands(t *testing.T, pwd string) []string {
	cmd := exec.Command("yarn", "run", "--json")
	cmd.Dir = pwd
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("Got error determining valid commands: %s", err)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	parsed := []map[string]interface{}{}
	for {
		var m map[string]interface{}
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			t.FailNow()
		}
		parsed = append(parsed, m)
	}
	var cmds []string

	addProvidedCmds := func(c map[string]interface{}) {
		// If this fails, we want the test to fail. We don't want to accidentally skip tests.
		data := c["data"].(map[string]interface{})
		if data["type"] == "possibleCommands" {
			return
		}
		for _, cmd := range data["items"].([]interface{}) {
			cmds = append(cmds, cmd.(string))
		}
	}

	addBinaryCmds := func(c map[string]interface{}) {
		data := c["data"].(string)
		if !strings.HasPrefix(data, "Commands available from binary scripts:") {
			return
		}
		cmdList := data[strings.Index(data, ":")+1:]
		for _, cmd := range strings.Split(cmdList, ",") {
			cmds = append(cmds, strings.TrimSpace(cmd))
		}
	}

	for _, c := range parsed {
		switch c["type"] {
		case "list":
			addProvidedCmds(c)
		case "info":
			addBinaryCmds(c)
		}
	}
	t.Logf("Found yarn commands in %s: %v", pwd, cmds)
	return cmds
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
	})
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isStringType(tt.input); got != tt.expected {
				t.Errorf("isStringType() = %v, want %v", got, tt.expected)
			}
		})
	}
}
