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

package pcl_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// resourceRefLoader serves a package with a custom resource and a component whose properties
// are plain references to that resource, alone, in a list, and in a map.
var resourceRefLoader = mockLoader{func() schema.PackageSpec {
	ref := schema.TypeSpec{Plain: true, Ref: "#/resources/refs:index:Custom"}
	properties := map[string]schema.PropertySpec{
		"resource":     {TypeSpec: ref},
		"resourceList": {TypeSpec: schema.TypeSpec{Plain: true, Type: "array", Items: &ref}},
		"resourceMap":  {TypeSpec: schema.TypeSpec{Plain: true, Type: "object", AdditionalProperties: &ref}},
	}
	return schema.PackageSpec{
		Name:    "refs",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"refs:index:Custom": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]schema.PropertySpec{"value": {TypeSpec: schema.TypeSpec{Type: "string"}}},
				},
				InputProperties: map[string]schema.PropertySpec{"value": {TypeSpec: schema.TypeSpec{Type: "string"}}},
			},
			"refs:index:Component": {
				IsComponent:     true,
				ObjectTypeSpec:  schema.ObjectTypeSpec{Type: "object", Properties: properties},
				InputProperties: properties,
			},
		},
	}
}()}

// TestBindPlainResourceReferences checks that a resource converts to a plain property typed as a
// reference to its resource type.
func TestBindPlainResourceReferences(t *testing.T) {
	t.Parallel()

	source := `
resource "custom1" "refs:index:Custom" {
    value = "hello"
}

resource "custom2" "refs:index:Custom" {
    value = "world"
}

resource "component" "refs:index:Component" {
    resource = custom1
    resourceList = [custom1, custom2]
    resourceMap = {
        "one" = custom1,
        "two" = custom2,
    }
}
`
	parser := syntax.NewParser()
	require.NoError(t, parser.ParseFile(strings.NewReader(source), "main.pp"))
	require.False(t, parser.Diagnostics.HasErrors(), parser.Diagnostics.Error())
	_, diags, err := pcl.BindProgram(parser.Files, resourceRefLoader)
	require.NoError(t, err)
	assert.Empty(t, diags)
}
