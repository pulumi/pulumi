// Copyright 2016-2026, Pulumi Corporation.
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

package nodejs

import (
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extensionSchemaSpec is a minimal extension-parameterized package: it carries
// ExtensionParameterization rather than a replacement Parameterization.
func extensionSchemaSpec() schema.PackageSpec {
	return schema.PackageSpec{
		Name:    "gateway",
		Version: "1.0.0",
		ExtensionParameterization: &schema.ParameterizationSpec{
			Name:         "gateway",
			BaseProvider: schema.BaseProviderSpec{Name: "kubernetes", Version: "4.0.0"},
			Parameter:    []byte("extension-parameter"),
		},
		Resources: map[string]schema.ResourceSpec{
			"gateway:index:Gateway": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]schema.PropertySpec{"name": {TypeSpec: schema.TypeSpec{Type: "string"}}},
				},
				InputProperties: map[string]schema.PropertySpec{"name": {TypeSpec: schema.TypeSpec{Type: "string"}}},
			},
		},
	}
}

// TestExtensionParameterizationCodegen checks that an extension-parameterized
// schema generates a Node.js SDK whose getPackage() registers an extension.
func TestExtensionParameterizationCodegen(t *testing.T) {
	t.Parallel()

	pkg, diags, err := schema.BindSpec(extensionSchemaSpec(), nil, schema.ValidationOptions{})
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), "%v", diags)
	require.NotNil(t, pkg.ExtensionParameterization)

	files, err := GeneratePackage("test", pkg, nil, nil, false, nil)
	require.NoError(t, err)

	var utilities string
	for path, content := range files {
		if strings.HasSuffix(path, "utilities.ts") {
			utilities = string(content)
		}
	}
	require.NotEmpty(t, utilities, "expected a utilities.ts in the generated SDK")
	assert.Contains(t, utilities, "extension: true",
		"an extension SDK's getPackage() should pass extension: true")
	assert.NotContains(t, utilities, "extension: false",
		"an extension SDK should not register a replacement parameterization")
}
