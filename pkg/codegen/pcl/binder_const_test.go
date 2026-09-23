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

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

var constLoader = mockLoader{func() schema.PackageSpec {
	spec := schema.PackageSpec{
		Name:    "constant",
		Version: "1.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"constant:index:Level": {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "integer"},
				Enum:           []schema.EnumValueSpec{{Value: 1}, {Value: 2}},
			},
		},
	}
	// The binder only reports an input type mismatch for a property the resource also declares
	// as an output.
	properties := map[string]schema.PropertySpec{
		"flag":  {TypeSpec: schema.TypeSpec{Type: "boolean"}, Const: true},
		"count": {TypeSpec: schema.TypeSpec{Type: "integer"}, Const: 3},
		"level": {TypeSpec: schema.TypeSpec{Ref: "#/types/constant:index:Level"}},
	}
	spec.Resources = map[string]schema.ResourceSpec{
		"constant:index:Resource": {
			ObjectTypeSpec:  schema.ObjectTypeSpec{Type: "object", Properties: properties},
			InputProperties: properties,
		},
	}
	return spec
}()}

// TestBindConstantAndEnumLiterals checks that a literal assigned to a constant or enum property
// binds only when it is that constant or a member of that enum.
func TestBindConstantAndEnumLiterals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		detail string
	}{
		{name: "matching bool constant", input: "flag = true"},
		{
			name:   "other bool constant",
			input:  "flag = false",
			detail: `Cannot assign value false to attribute of type "Optional<boolean>" for resource "constant::Resource"`,
		},
		{name: "matching int constant", input: "count = 3"},
		{
			name:   "other int constant",
			input:  "count = 4",
			detail: `Cannot assign value 4 to attribute of type "Optional<integer>" for resource "constant::Resource"`,
		},
		{name: "enum member", input: "level = 2"},
		{
			name:  "enum non-member",
			input: "level = 3",
			detail: `Cannot assign value 3 to attribute of type "Optional<constant:index:Level>" ` +
				`for resource "constant::Resource"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			source := "resource \"r\" \"constant:index:Resource\" {\n  " + c.input + "\n}\n"
			parser := syntax.NewParser()
			require.NoError(t, parser.ParseFile(strings.NewReader(source), "main.pp"))
			require.False(t, parser.Diagnostics.HasErrors(), parser.Diagnostics.Error())
			_, diags, err := pcl.BindProgram(parser.Files, constLoader)
			if c.detail == "" {
				require.NoError(t, err)
				assert.Empty(t, diags)
				return
			}
			require.Error(t, err)
			require.Len(t, diags, 1)
			assert.Equal(t, hcl.DiagError, diags[0].Severity)
			assert.Equal(t, c.detail, diags[0].Detail)
		})
	}
}
