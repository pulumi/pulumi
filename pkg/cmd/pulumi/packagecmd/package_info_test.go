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
	"strings"
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
	require.Equal(t, fmt.Sprintf(`\x1b[1mName\x1b[0m: test
\x1b[1mVersion\x1b[0m: 0.0.1
\x1b[1mDescription\x1b[0m: test description markdown formatted
\x1b[1mTotal resources\x1b[0m 3
\x1b[1mTotal modules\x1b[0m: 2

\x1b[1mModules\x1b[0m: another, index

Use 'pulumi package info %[1]s --module <module>' to list resources in a module
Use 'pulumi package info %[1]s --resource <resource>  --module <module>' for detailed resource info
`, schemaPath), strings.ReplaceAll(output.String(), "\x1b", "\\x1b"))
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
	require.Equal(t, `\x1b[1mName\x1b[0m: test
\x1b[1mVersion\x1b[0m: 0.0.1
\x1b[1mDescription\x1b[0m: test description markdown formatted
\x1b[1mResources\x1b[0m: 2

 - \x1b[1mTest\x1b[0m: test resource description
 - \x1b[1mTest2\x1b[0m: this is another test resource
`, strings.ReplaceAll(output.String(), "\x1b", "\\x1b"))
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
	require.Equal(t, `\x1b[1mResource\x1b[0m: test:index:Test
\x1b[1mDescription\x1b[0m: test resource description

\x1b[1mInputs\x1b[0m:
 - \x1b[1mprop1\x1b[0m (\x1b[4mstring\x1b[0m\x1b[4m*\x1b[0m): this is a string property
Inputs marked with '*' are required

\x1b[1mOutputs\x1b[0m:
(All input properties are implicitly available as output properties)
 - \x1b[1marrayProp\x1b[0m (\x1b[4m[]TestType\x1b[0m\x1b[4m\x1b[0m): this is an array property
 - \x1b[1menumProp\x1b[0m (\x1b[4menum(string){EnumValue1, value2}\x1b[0m\x1b[4m\x1b[0m): this is an enum property
 - \x1b[1mmapProp\x1b[0m (\x1b[4mmap[string]string\x1b[0m\x1b[4m\x1b[0m): this is a map property
`, strings.ReplaceAll(output.String(), "\x1b", "\\x1b"))
}
