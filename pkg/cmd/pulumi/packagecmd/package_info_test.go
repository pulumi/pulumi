// Copyright 2025, Pulumi Corporation.
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

package packagecmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/adder"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/stretchr/testify/require"
)

// testSpindle needs no login manager: these tests load schemas from a file
// path, so the lazy registry — the only thing that would resolve a backend —
// is never queried.
func testSpindle() adder.Spindle {
	return adder.Spindle{WS: &pkgWorkspace.MockContext{}}
}

func generateSchema(t *testing.T) []byte {
	spec := &schema.PackageSpec{
		Name:    "test",
		Version: "0.0.1",
		Description: `test description
markdown formatted

another paragraph`,
		Resources: map[string]schema.ResourceSpec{
			"test:index:Test": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "test\nresource\ndescription\n\nanother paragraph",
					Properties: map[string]schema.PropertySpec{
						"prop1": {
							Description: "this is a string property",
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"arrayProp": {
							Description: "this is an array property",
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Ref: "#/types/test:index:TestType",
								},
							},
						},
						"enumProp": {
							Description: "this is an enum property",
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/test:index:EnumType",
							},
						},
						"mapProp": {
							Description: "this is a map property",
							TypeSpec: schema.TypeSpec{
								Type: "object",
								AdditionalProperties: &schema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
					Required: []string{"prop1"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"prop1": {
						Description: "this is a string property",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
			"test:index:Test2": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "this is another test resource",
				},
				InputProperties: map[string]schema.PropertySpec{
					"propA": {
						Description: "this is propA",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
				RequiredInputs: []string{"propA"},
			},
			"test:another/Test:Test": {},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"test:index:TestType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "this is a test type",
					Type:        "object",
				},
			},
			"test:index:EnumType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "this is an enum type",
					Type:        "string",
				},
				Enum: []schema.EnumValueSpec{
					{
						Name:  "EnumValue1",
						Value: "value1",
					},
					{
						Value: "value2",
					},
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"test:funs:TestFunction": {
				Description: "this is a test function",
				Inputs: &schema.ObjectTypeSpec{
					Description: "properties for TestFunction",
					Properties: map[string]schema.PropertySpec{
						"input1": {
							Description: "the first and only input",
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
				},
				ReturnType: &schema.ReturnTypeSpec{
					TypeSpec: &schema.TypeSpec{
						Type: "string",
					},
				},
			},
			"test:funs:TestFunction2": {
				Inputs: &schema.ObjectTypeSpec{
					Description: "properties for TestFunction2",
					Properties: map[string]schema.PropertySpec{
						"input1": {
							Description: "a flag input",
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
						},
					},
					Required: []string{"input1"},
				},
				Outputs: &schema.ObjectTypeSpec{
					Description: "the outputs for TestFunction2",
					Properties: map[string]schema.PropertySpec{
						"output1": {
							Description: "the first and only output",
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}
	marshalled, err := json.Marshal(spec)
	require.NoError(t, err)
	return marshalled
}

func TestPackageInfo(t *testing.T) {
	t.Parallel()

	schema := generateSchema(t)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	err := os.WriteFile(schemaPath, schema, 0o600)
	require.NoError(t, err)

	cmd := newPackageInfoCmd(testSpindle())
	cmd.SetContext(adder.WithBag(t.Context()))
	cmd.SetArgs([]string{schemaPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf(`Name: test
Version: 0.0.1
Description: test description markdown formatted
Total resources 3
Total functions 2
Total modules: 3

Modules: another, funs, index

Use 'pulumi package info %[1]s --module <module>' to list resources in a module
Use 'pulumi package info %[1]s --module <module> --resource <resource>' for detailed resource info
`, schemaPath), output.String())
}

func TestModuleInfo(t *testing.T) {
	t.Parallel()

	schema := generateSchema(t)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	err := os.WriteFile(schemaPath, schema, 0o600)
	require.NoError(t, err)

	cmd := newPackageInfoCmd(testSpindle())
	cmd.SetContext(adder.WithBag(t.Context()))
	cmd.SetArgs([]string{"--module", "index", schemaPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, `Name: test
Module: index
Version: 0.0.1
Description: test description markdown formatted
Resources: 2

 - Test: test resource description
 - Test2: this is another test resource

Functions: 0

`, output.String())
}

func TestResourceInfo(t *testing.T) {
	t.Parallel()

	schema := generateSchema(t)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	err := os.WriteFile(schemaPath, schema, 0o600)
	require.NoError(t, err)
	cmd := newPackageInfoCmd(testSpindle())
	cmd.SetContext(adder.WithBag(t.Context()))
	cmd.SetArgs([]string{"--module", "index", "--resource", "Test", schemaPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, `Resource: test:index:Test
Description: test resource description

Inputs:
 - prop1 (string): this is a string property

Outputs:
 - arrayProp (Array<test:index:TestType>): this is an array property
 - enumProp (test:index:EnumType): this is an enum property
 - mapProp (Map<string>): this is a map property
 - prop1 (string*): this is a string property
Outputs marked with '*' are always present
`, output.String())

	cmd.SetArgs([]string{"--module", "index", "--resource", "Test2", schemaPath})
	output.Reset()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, `Resource: test:index:Test2
Description: this is another test resource

Inputs:
 - propA (string*): this is propA
Inputs marked with '*' are required

Outputs:
`, output.String())
}

func TestFunctionInfo(t *testing.T) {
	t.Parallel()

	schema := generateSchema(t)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	err := os.WriteFile(schemaPath, schema, 0o600)
	require.NoError(t, err)

	cmd := newPackageInfoCmd(testSpindle())
	cmd.SetContext(adder.WithBag(t.Context()))
	cmd.SetArgs([]string{"--module", "funs", "--function", "TestFunction", schemaPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, `Function: test:funs:TestFunction
Description: this is a test function

Inputs:
 - input1 (string): the first and only input

Outputs: string
`, output.String())

	cmd.SetArgs([]string{"--function", "TestFunction2", schemaPath})
	output = bytes.Buffer{}
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "Function: test:funs:TestFunction2\n"+
		"Description: \n"+
		"\n"+
		"Inputs:\n"+
		" - input1 (boolean*): a flag input\n"+
		"Inputs marked with '*' are required\n"+
		"\n"+
		"Outputs:\n"+
		" - output1 (string): the first and only output\n", output.String())
}
