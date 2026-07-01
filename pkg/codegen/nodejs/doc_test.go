// Copyright 2016, Pulumi Corporation.
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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:lll, goconst
package nodejs

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

var testPackageSpec = schema.PackageSpec{
	Name:        "aws",
	Version:     "0.0.1",
	Description: "A fake provider package used for testing.",
	Meta: &schema.MetadataSpec{
		ModuleFormat: "(.*)(?:/[^/]*)",
	},
	Types: map[string]schema.ComplexTypeSpec{
		"aws:s3/BucketCorsRule:BucketCorsRule": {
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Description: "The resource options object.",
				Type:        "object",
				Properties: map[string]schema.PropertySpec{
					"stringProp": {
						Description: "A string prop.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
	},
	Resources: map[string]schema.ResourceSpec{
		"aws:s3/bucket:Bucket": {
			InputProperties: map[string]schema.PropertySpec{
				"corsRules": {
					TypeSpec: schema.TypeSpec{
						Ref: "#/types/aws:s3/BucketCorsRule:BucketCorsRule",
					},
				},
			},
		},
	},
}

func getTestPackage(t *testing.T) *schema.Package {
	t.Helper()

	pkg, err := schema.ImportSpec(testPackageSpec, nil, schema.NewNullLoader(), schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err, "could not import the test package spec")
	return pkg
}

func TestDocLinkGenerationForPulumiTypes(t *testing.T) {
	t.Parallel()

	pkg := getTestPackage(t)
	d := DocLanguageHelper{}
	t.Run("GenerateCustomResourceOptionsLink", func(t *testing.T) {
		t.Parallel()

		expected := "/docs/reference/pkg/nodejs/pulumi/pulumi/#CustomResourceOptions"
		link := d.GetDocLinkForPulumiType(pkg, "CustomResourceOptions")
		assert.Equal(t, expected, link)
	})
	t.Run("GenerateInvokeOptionsLink", func(t *testing.T) {
		t.Parallel()

		expected := "/docs/reference/pkg/nodejs/pulumi/pulumi/#InvokeOptions"
		link := d.GetDocLinkForPulumiType(pkg, "InvokeOptions")
		assert.Equal(t, expected, link)
	})
}

func TestGetDocLinkForResourceType(t *testing.T) {
	t.Parallel()

	pkg := getTestPackage(t)
	d := DocLanguageHelper{}
	expected := "/docs/reference/pkg/nodejs/pulumi/aws/s3/#Bucket"
	link := d.GetDocLinkForResourceType(pkg, "s3", "Bucket")
	assert.Equal(t, expected, link)
}

func TestResolveDocRef(t *testing.T) {
	t.Parallel()

	pkg := getTestPackage(t)
	d := DocLanguageHelper{}

	cases := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "resource",
			description: "See {{% ref #/resources/aws:s3%2Fbucket:Bucket %}}.",
			expected:    "See Bucket.",
		},
		{
			name:        "resource input property",
			description: "See {{% ref #/resources/aws:s3%2Fbucket:Bucket/inputProperties/corsRules %}}.",
			expected:    "See BucketArgs.corsRules.",
		},
		{
			name:        "type",
			description: "See {{% ref #/types/aws:s3%2FBucketCorsRule:BucketCorsRule %}}.",
			expected:    "See BucketCorsRule.",
		},
		{
			name:        "type property",
			description: "See {{% ref #/types/aws:s3%2FBucketCorsRule:BucketCorsRule/properties/stringProp %}}.",
			expected:    "See BucketCorsRule.stringProp.",
		},
		{
			name:        "empty description",
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

	t.Run("selfRef", func(t *testing.T) {
		t.Parallel()
		fixture, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "demo",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"demo:mod/widget:Widget": {
					InputProperties: map[string]schema.PropertySpec{
						"size": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"size":  {TypeSpec: schema.TypeSpec{Type: "string"}},
							"color": {TypeSpec: schema.TypeSpec{Type: "string"}},
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
							"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"id": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)

		// Locate fixtures by token (slice order is not guaranteed).
		var widget *schema.Resource
		for _, r := range fixture.Resources {
			if r.Token == "demo:mod/widget:Widget" {
				widget = r
			}
		}
		require.NotNil(t, widget)
		var settings schema.Type
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

		cases := []struct {
			name        string
			selfRef     schema.DocRef
			description string
			expected    string
		}{
			{
				name:        "resource self-property unqualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/resources/demo:mod%2Fwidget:Widget/properties/color %}}.",
				expected:    "See color.",
			},
			{
				name:        "resource self-input-property unqualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/resources/demo:mod%2Fwidget:Widget/inputProperties/size %}}.",
				expected:    "See size.",
			},
			{
				name:        "resource ref to other entity stays qualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/types/demo:mod%2FSettings:Settings/properties/verbose %}}.",
				expected:    "See Settings.verbose.",
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
				description: "See {{% ref #/functions/demo:mod:getWidget/inputs/properties/name %}}.",
				expected:    "See name.",
			},
			{
				name:        "function self-output-property unqualified",
				selfRef:     schema.DocRefForFunction(getWidget),
				description: "See {{% ref #/functions/demo:mod:getWidget/outputs/properties/id %}}.",
				expected:    "See id.",
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
}

func TestGetDocLinkForResourceInputOrOutputType(t *testing.T) {
	t.Parallel()

	pkg := getTestPackage(t)
	d := DocLanguageHelper{}
	expected := "/docs/reference/pkg/nodejs/pulumi/aws/types/input/#BucketCorsRule"
	link := d.GetDocLinkForResourceInputOrOutputType(pkg, "s3", "BucketCorsRule", true)
	assert.Equal(t, expected, link)
}
