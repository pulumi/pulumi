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

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
)

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

	cmd := newPackageInfoCmd()
	cmd.SetArgs([]string{schemaPath})
	var output bytes.Buffer
	cmd.SetOutput(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf(`Provider: test (0.0.1)
  Description: test description markdown formatted
Total resources: 3
Total modules: 2

Use 'pulumi package info --module <module> %[1]s' to list resources in a module
Use 'pulumi package info --module <module> --resource <resource> %[1]s' for detailed resource info
`, schemaPath), output.String())
}

func TestModuleInfo(t *testing.T) {
	t.Parallel()

	schema := generateSchema(t)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	err := os.WriteFile(schemaPath, schema, 0o600)
	require.NoError(t, err)

	cmd := newPackageInfoCmd()
	cmd.SetArgs([]string{"--module", "index", schemaPath})
	var output bytes.Buffer
	cmd.SetOutput(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, `Module: test:index
Resources: 2

 - Test: test resource description
 - Test2: this is another test resource
`, output.String())
}

func TestResourceInfo(t *testing.T) {
	t.Parallel()

	schema := generateSchema(t)
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")

	err := os.WriteFile(schemaPath, schema, 0o600)
	require.NoError(t, err)
	cmd := newPackageInfoCmd()
	cmd.SetArgs([]string{"--module", "index", "--resource", "Test", schemaPath})
	var output bytes.Buffer
	cmd.SetOutput(&output)
	err = cmd.Execute()
	require.NoError(t, err)
	require.Equal(t, `Resource: test:index:Test

Input Properties:
 - prop1 (string (optional)): this is a string property

Output Properties:
 - arrayProp ([]TestType): this is an array property
 - enumProp (enum(string){EnumValue1, value2}): this is an enum property
 - prop1 (string (always present)): this is a string property
`, output.String())
}
