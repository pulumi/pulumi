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

package python

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Properties marked secret on a nested object type must be wrapped in pulumi.Output.secret in the generated input
// type, the same way resource inputs are.
func TestNestedInputTypeSecretProperties(t *testing.T) {
	t.Parallel()

	const spec = `
	{
		"name": "vault",
		"version": "0.0.1",
		"meta": {
			"moduleFormat": "(.*)(?:/[^/]*)"
		},
		"types": {
			"vault:database/connection:Connection": {
				"type": "object",
				"properties": {
					"host": {"type": "string"},
					"password": {"type": "string", "secret": true},
					"token": {"type": "string", "secret": true}
				},
				"required": ["token"]
			}
		},
		"resources": {
			"vault:database/backend:Backend": {
				"properties": {
					"connection": {"$ref": "#/types/vault:database/connection:Connection"}
				},
				"inputProperties": {
					"connection": {"$ref": "#/types/vault:database/connection:Connection"}
				}
			}
		},
		"language": {
			"python": {
				"readme": ".."
			}
		}
	}`

	var pkgSpec schema.PackageSpec
	require.NoError(t, json.Unmarshal([]byte(spec), &pkgSpec))

	pkg, err := schema.ImportSpec(pkgSpec, nil, schema.NewNullLoader(), schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err)

	files, err := GeneratePackage("tool", pkg, nil, nil)
	require.NoError(t, err)

	inputs := string(files["pulumi_vault/database/_inputs.py"])
	require.NotEmpty(t, inputs, "expected pulumi_vault/database/_inputs.py to be generated")

	assert.Contains(t, inputs, `pulumi.set(__self__, "password", pulumi.Output.secret(password))`)
	assert.Contains(t, inputs, `pulumi.set(__self__, "token", pulumi.Output.secret(token))`)
	assert.Contains(t, inputs, `pulumi.set(__self__, "host", host)`)

	// The secret wrapper only belongs on inputs, outputs already come back from the engine as secrets.
	outputs := string(files["pulumi_vault/database/outputs.py"])
	require.NotEmpty(t, outputs, "expected pulumi_vault/database/outputs.py to be generated")
	assert.False(t, strings.Contains(outputs, "pulumi.Output.secret(password)"),
		"output types must not wrap secrets")
}
