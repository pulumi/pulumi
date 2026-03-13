// Copyright 2026, Pulumi Corporation.
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

package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pkgSchema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"
)

func validSchemaJSON(t *testing.T) []byte {
	t.Helper()
	spec := &pkgSchema.PackageSpec{
		Name:    "test-provider",
		Version: "0.1.0",
		Resources: map[string]pkgSchema.ResourceSpec{
			"test-provider:index:MyResource": {
				ObjectTypeSpec: pkgSchema.ObjectTypeSpec{
					Properties: map[string]pkgSchema.PropertySpec{
						"name": {
							TypeSpec: pkgSchema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"name"},
				},
				InputProperties: map[string]pkgSchema.PropertySpec{
					"name": {
						TypeSpec: pkgSchema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"name"},
			},
		},
	}
	b, err := json.Marshal(spec)
	require.NoError(t, err)
	return b
}

func danglingRefSchemaJSON(t *testing.T) []byte {
	t.Helper()
	spec := &pkgSchema.PackageSpec{
		Name:    "test-provider",
		Version: "0.1.0",
		Resources: map[string]pkgSchema.ResourceSpec{
			"test-provider:index:MyResource": {
				ObjectTypeSpec: pkgSchema.ObjectTypeSpec{
					Properties: map[string]pkgSchema.PropertySpec{
						"config": {
							TypeSpec: pkgSchema.TypeSpec{
								Ref: "#/types/test-provider:index:NonExistentType",
							},
						},
					},
				},
				InputProperties: map[string]pkgSchema.PropertySpec{
					"config": {
						TypeSpec: pkgSchema.TypeSpec{
							Ref: "#/types/test-provider:index:NonExistentType",
						},
					},
				},
			},
		},
	}
	b, err := json.Marshal(spec)
	require.NoError(t, err)
	return b
}

func TestSchemaCheckFromStdin(t *testing.T) {
	t.Parallel()

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{"-"})
	cmd.SetIn(bytes.NewReader(validSchemaJSON(t)))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestSchemaCheckFromStdinInvalid(t *testing.T) {
	t.Parallel()

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{"-"})
	cmd.SetIn(bytes.NewReader([]byte("not valid json or yaml")))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal schema")
}

func TestSchemaCheckFromJSONFile(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	err := os.WriteFile(schemaPath, validSchemaJSON(t), 0o600)
	require.NoError(t, err)

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{schemaPath})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err = cmd.Execute()
	require.NoError(t, err)
}

func TestSchemaCheckFromYAMLFile(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join(t.TempDir(), "schema.yaml")
	err := os.WriteFile(schemaPath, []byte(`
name: test-provider
version: "0.1.0"
resources:
  test-provider:index:MyResource:
    properties:
      name:
        type: string
    required:
      - name
    inputProperties:
      name:
        type: string
    requiredInputs:
      - name
`), 0o600)
	require.NoError(t, err)

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{schemaPath})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err = cmd.Execute()
	require.NoError(t, err)
}

func TestSchemaCheckDanglingRefFails(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	err := os.WriteFile(schemaPath, danglingRefSchemaJSON(t), 0o600)
	require.NoError(t, err)

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{schemaPath})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err = cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func TestSchemaCheckDanglingRefAllowed(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	err := os.WriteFile(schemaPath, danglingRefSchemaJSON(t), 0o600)
	require.NoError(t, err)

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{"--allow-dangling-references", schemaPath})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err = cmd.Execute()
	require.NoError(t, err)
}

func TestSchemaCheckStdinYAML(t *testing.T) {
	t.Parallel()

	yamlSchema := []byte(`
name: test-provider
version: "0.1.0"
resources:
  test-provider:index:MyResource:
    properties:
      name:
        type: string
    required:
      - name
    inputProperties:
      name:
        type: string
    requiredInputs:
      - name
`)

	cmd := newSchemaCheckCommand()
	cmd.SetArgs([]string{"-"})
	cmd.SetIn(bytes.NewReader(yamlSchema))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.NoError(t, err)
}
