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

var quotedLoader = mockLoader{schema.PackageSpec{
	Name:    "quoted",
	Version: "1.0.0",
	Types: map[string]schema.ComplexTypeSpec{
		"quoted:index:Inner": {ObjectTypeSpec: schema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]schema.PropertySpec{
				"bar":  {TypeSpec: schema.TypeSpec{Type: "string"}},
				"baz!": {TypeSpec: schema.TypeSpec{Type: "string"}},
			},
		}},
	},
	Resources: map[string]schema.ResourceSpec{
		"quoted:index:Resource": {
			InputProperties: map[string]schema.PropertySpec{
				"foo!":  {TypeSpec: schema.TypeSpec{Ref: "#/types/quoted:index:Inner"}},
				"plain": {TypeSpec: schema.TypeSpec{Type: "string"}},
			},
		},
	},
}}

// TestBindQuotedProperties checks that the `_` attribute sets properties by quoted name, and that `_` roots a
// property path by quoted name.
func TestBindQuotedProperties(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
		// The input names of the resource or component, in order, if binding succeeds.
		inputs []string
		// The value of the ignoreChanges option of the resource.
		ignoreChanges []string
		// The summaries of the diagnostics.
		diagnostics []string
	}{
		{
			name: "quoted name",
			source: `resource "r" "quoted:index:Resource" {
	plain = "a"
	_ = { "foo!" = { "baz!" = "b" } }
}`,
			inputs: []string{"plain", "foo!"},
		},
		{
			name: "identifier name under _",
			source: `resource "r" "quoted:index:Resource" {
	_ = { "plain" = "a" }
}`,
			inputs: []string{"plain"},
		},
		{
			name: "name set directly and under _",
			source: `resource "r" "quoted:index:Resource" {
	plain = "a"
	_ = { "plain" = "b" }
}`,
			diagnostics: []string{"duplicate attribute 'plain'"},
		},
		{
			name: "name set twice under _",
			source: `resource "r" "quoted:index:Resource" {
	_ = { "foo!" = {}, "foo!" = {} }
}`,
			diagnostics: []string{"duplicate attribute 'foo!'"},
		},
		{
			name: "unknown name under _",
			source: `resource "r" "quoted:index:Resource" {
	_ = { "nope!" = "a" }
}`,
			diagnostics: []string{"unsupported attribute 'nope!'"},
		},
		{
			name: "_ is not an object literal",
			source: `resource "r" "quoted:index:Resource" {
	_ = "a"
}`,
			diagnostics: []string{"attribute '_' must be an object literal"},
		},
		{
			name: "key under _ is not a literal",
			source: `k = "plain"
resource "r" "quoted:index:Resource" {
	_ = { (k) = "a" }
}`,
			diagnostics: []string{"the keys of attribute '_' must be non-empty string literals"},
		},
		{
			name: "property paths",
			source: `resource "r" "quoted:index:Resource" {
	options {
		ignoreChanges = [plain, _["foo!"].bar, _["foo!"]["baz!"], _["plain"]]
	}
}`,
			inputs:        []string{},
			ignoreChanges: []string{"plain", `["foo!"].bar`, `["foo!"]["baz!"]`, "plain"},
		},
		{
			name: "property path of _ alone",
			source: `resource "r" "quoted:index:Resource" {
	options {
		ignoreChanges = [_]
	}
}`,
			diagnostics: []string{"cannot assign expression of type " +
				"({ foo!: { bar: string?, baz!: string? }?, plain: string? }) " +
				"to location of type list(Property | output(Property)) | output(list(Property)): "},
		},
		{
			name: "property path of an unknown quoted name",
			source: `resource "r" "quoted:index:Resource" {
	options {
		ignoreChanges = [_["nope!"]]
	}
}`,
			diagnostics: []string{"unknown property 'nope!' among [foo! plain]"},
		},
		{
			name: "sourceless component",
			source: `component "c" {
	token = "my:custom:Component"
	plain = 1
	_ = { "foo!" = 2 }
}`,
			inputs: []string{"plain", "foo!"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			parser := syntax.NewParser()
			require.NoError(t, parser.ParseFile(strings.NewReader(c.source), "main.pp"))
			require.False(t, parser.Diagnostics.HasErrors(), parser.Diagnostics.Error())
			program, diags, _ := pcl.BindProgram(parser.Files, quotedLoader)

			summaries := make([]string, 0, len(diags))
			for _, d := range diags {
				summaries = append(summaries, d.Summary)
			}
			if c.diagnostics == nil {
				c.diagnostics = []string{}
			}
			assert.Equal(t, c.diagnostics, summaries)
			if len(c.diagnostics) > 0 {
				return
			}

			inputs := []string{}
			var ignoreChanges []string
			for _, node := range program.Nodes {
				switch node := node.(type) {
				case *pcl.Resource:
					for _, input := range node.Inputs {
						inputs = append(inputs, input.Name)
					}
					if node.Options != nil && node.Options.IgnoreChanges != nil {
						value, valueDiags := node.Options.IgnoreChanges.Evaluate(nil)
						require.Empty(t, valueDiags)
						for _, v := range value.AsValueSlice() {
							ignoreChanges = append(ignoreChanges, v.AsString())
						}
					}
				case *pcl.Component:
					for _, input := range node.Inputs {
						inputs = append(inputs, input.Name)
					}
				}
			}
			assert.Equal(t, c.inputs, inputs)
			assert.Equal(t, c.ignoreChanges, ignoreChanges)
		})
	}
}
