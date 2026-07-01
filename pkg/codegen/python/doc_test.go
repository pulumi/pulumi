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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderWithResolver is a test helper that obtains a DocRefResolver and uses it to substitute the
// ref shortcodes in description, mirroring what a doc-generating caller would do (modulo its own
// AST walking / code-block handling).
func renderWithResolver(
	t *testing.T, d DocLanguageHelper, pkg schema.PackageReference,
	selfRef schema.DocRef, description string,
) string {
	t.Helper()
	rendered, err := pkg.InterpretPulumiRefs(description, func(ref schema.DocRef) (string, bool) {
		name, ok, err := d.ResolveDocRef(pkg, selfRef, ref)
		require.NoError(t, err)
		return name, ok
	})
	require.NoError(t, err)
	return strings.TrimSuffix(rendered, "\n")
}

func TestResolveDocRef(t *testing.T) {
	t.Parallel()

	t.Run("basic refs", func(t *testing.T) {
		t.Parallel()

		pkg, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "aws",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Types: map[string]schema.ComplexTypeSpec{
				"aws:s3/BucketCorsRule:BucketCorsRule": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"stringProp": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:s3/bucket:Bucket": {
					InputProperties: map[string]schema.PropertySpec{
						"corsRules": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)
		d := DocLanguageHelper{}

		cases := []struct {
			name        string
			description string
			expected    string
		}{
			{
				name:        "resource",
				description: "See {{% ref #/resources/aws:s3%2Fbucket:Bucket %}}.",
				expected:    "See _s3.Bucket.",
			},
			{
				name:        "resource input property",
				description: "See {{% ref #/resources/aws:s3%2Fbucket:Bucket/inputProperties/corsRules %}}.",
				expected:    "See _s3.BucketArgs.cors_rules.",
			},
			{
				name:        "type",
				description: "See {{% ref #/types/aws:s3%2FBucketCorsRule:BucketCorsRule %}}.",
				expected:    "See BucketCorsRuleArgs.",
			},
			{
				name:        "type property",
				description: "See {{% ref #/types/aws:s3%2FBucketCorsRule:BucketCorsRule/properties/stringProp %}}.",
				expected:    "See BucketCorsRuleArgs.string_prop.",
			},
			{
				name:        "empty",
				description: "",
				expected:    "",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := renderWithResolver(t, d, pkg.Reference(), schema.DocRef{}, tc.description)
				assert.Equal(t, tc.expected, got)
			})
		}
	})

	t.Run("selfRef", func(t *testing.T) {
		t.Parallel()
		fixture, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "demo",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"demo:mod/widget:Widget": {
					InputProperties: map[string]schema.PropertySpec{
						"sizeProp": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"sizeProp":  {TypeSpec: schema.TypeSpec{Type: "string"}},
							"colorProp": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
			Types: map[string]schema.ComplexTypeSpec{
				"demo:mod/Settings:Settings": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"verbose": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"demo:mod:getWidget": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"nameArg": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"resultId": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)

		var widget *schema.Resource
		for _, r := range fixture.Resources {
			if r.Token == "demo:mod/widget:Widget" {
				widget = r
			}
		}
		require.NotNil(t, widget)
		var settings *schema.ObjectType
		for _, typ := range fixture.Types {
			if obj, ok := typ.(*schema.ObjectType); ok && obj.Token == "demo:mod/Settings:Settings" {
				settings = obj
			}
		}
		require.NotNil(t, settings)
		var getWidget *schema.Function
		for _, f := range fixture.Functions {
			if f.Token == "demo:mod:getWidget" {
				getWidget = f
			}
		}
		require.NotNil(t, getWidget)

		d := DocLanguageHelper{}
		cases := []struct {
			name        string
			selfRef     schema.DocRef
			description string
			expected    string
		}{
			{
				name:        "resource self-property unqualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/resources/demo:mod%2Fwidget:Widget/properties/colorProp %}}.",
				expected:    "See color_prop.",
			},
			{
				name:        "resource self-input-property unqualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/resources/demo:mod%2Fwidget:Widget/inputProperties/sizeProp %}}.",
				expected:    "See size_prop.",
			},
			{
				name:        "resource ref to other entity stays qualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/types/demo:mod%2FSettings:Settings/properties/verbose %}}.",
				expected:    "See SettingsArgs.verbose.",
			},
			{
				name:        "type self-property unqualified",
				selfRef:     schema.DocRefForType(settings),
				description: "See {{% ref #/types/demo:mod%2FSettings:Settings/properties/verbose %}}.",
				expected:    "See verbose.",
			},
			{
				name:        "function self-input-property unqualified",
				selfRef:     schema.DocRefForFunction(getWidget),
				description: "See {{% ref #/functions/demo:mod:getWidget/inputs/properties/nameArg %}}.",
				expected:    "See name_arg.",
			},
			{
				name:        "function self-output-property unqualified",
				selfRef:     schema.DocRefForFunction(getWidget),
				description: "See {{% ref #/functions/demo:mod:getWidget/outputs/properties/resultId %}}.",
				expected:    "See result_id.",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := renderWithResolver(t, d, fixture.Reference(), tc.selfRef, tc.description)
				assert.Equal(t, tc.expected, got)
			})
		}
	})

	t.Run("InitArgs fallback for resource/object token collision", func(t *testing.T) {
		t.Parallel()

		// When an object type shares a token with a resource, genResource emits the args class
		// as `<Resource>InitArgs` instead of `<Resource>Args`. DocRefResolver must agree.
		pkg, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "dup",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"dup:idx:Thing": {
					InputProperties: map[string]schema.PropertySpec{
						"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
			},
			Types: map[string]schema.ComplexTypeSpec{
				"dup:idx:Thing": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)

		d := DocLanguageHelper{}
		got := renderWithResolver(t, d, pkg.Reference(), schema.DocRef{},
			"See {{% ref #/resources/dup:idx:Thing/inputProperties/name %}}.")
		assert.Equal(t, "See ThingInitArgs.name.", got)
	})

	t.Run("ModuleNameOverrides applied", func(t *testing.T) {
		t.Parallel()

		pkg, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "pkg",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"pkg:original/widget:Widget": {},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)
		pkg.Language["python"] = PackageInfo{
			ModuleNameOverrides: map[string]string{"original": "renamed"},
		}

		d := DocLanguageHelper{}
		got := renderWithResolver(t, d, pkg.Reference(), schema.DocRef{},
			"See {{% ref #/resources/pkg:original%2Fwidget:Widget %}}.")
		// The module override should turn the schema module "original" into the python module
		// "renamed" in the qualified reference.
		assert.Equal(t, "See _renamed.Widget.", got)
	})
}
