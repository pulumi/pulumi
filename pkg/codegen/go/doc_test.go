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
package gen

import (
	"fmt"
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

	pkg, err := schema.ImportSpec(testPackageSpec, nil, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err, "could not import the test package spec")
	return pkg
}

func TestGetDocLinkForPulumiType(t *testing.T) {
	t.Parallel()

	t.Run("Generate_ResourceOptionsLink_Specified", func(t *testing.T) {
		t.Parallel()

		pkg := getTestPackage(t)
		d := DocLanguageHelper{}
		pkg.Language["go"] = GoPackageInfo{PulumiSDKVersion: 1}
		expected := "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/go/pulumi?tab=doc#ResourceOption"
		link := d.GetDocLinkForPulumiType(pkg, "ResourceOption")
		assert.Equal(t, expected, link)
	})
	t.Run("Generate_ResourceOptionsLink_Specified", func(t *testing.T) {
		t.Parallel()

		pkg := getTestPackage(t)
		d := DocLanguageHelper{}
		pkg.Language["go"] = GoPackageInfo{PulumiSDKVersion: 2}
		expected := "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v2/go/pulumi?tab=doc#ResourceOption"
		link := d.GetDocLinkForPulumiType(pkg, "ResourceOption")
		assert.Equal(t, expected, link)
	})
	t.Run("Generate_ResourceOptionsLink_Unspecified", func(t *testing.T) {
		t.Parallel()

		pkg := getTestPackage(t)
		d := DocLanguageHelper{}
		expected := fmt.Sprintf("https://pkg.go.dev/github.com/pulumi/pulumi/sdk/%s/go/pulumi?tab=doc#ResourceOption", pulumiSDKVersion)
		link := d.GetDocLinkForPulumiType(pkg, "ResourceOption")
		assert.Equal(t, expected, link)
	})
}

func TestGetDocLinkForResourceType(t *testing.T) {
	t.Parallel()

	pkg := getTestPackage(t)
	d := DocLanguageHelper{}
	expected := "https://pkg.go.dev/github.com/pulumi/pulumi-aws/sdk/go/aws/s3?tab=doc#Bucket"
	link := d.GetDocLinkForResourceType(pkg, "s3", "Bucket")
	assert.Equal(t, expected, link)
}

func TestGetFunctionName(t *testing.T) {
	t.Parallel()
	pkg, err := schema.ImportSpec(schema.PackageSpec{
		Name:    "pkg",
		Version: "0.0.1",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Resources: map[string]schema.ResourceSpec{
			"pkg:conflict:Resource": {},
		},
		Functions: map[string]schema.FunctionSpec{
			"pkg:index:getSomeFunction": {},
			"pkg:conflict:newResource":  {},
		},
	}, nil, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err)
	d := DocLanguageHelper{}
	d.GeneratePackagesMap(pkg.Reference(), "test", GoPackageInfo{})

	names := map[string]string{}
	for _, f := range pkg.Functions {
		names[f.Token] = d.GetFunctionName(f)
	}

	assert.Equal(t, map[string]string{
		"pkg:index:getSomeFunction": "GetSomeFunction",
		// "pkg:conflict:newResource" is renamed to "CreateResource" to avoid
		// conflicting with the resource constructor for "pkg:conflict:Resource"
		// (NewResource).
		"pkg:conflict:newResource": "CreateResource",
	}, names)
}

// buildSelfRefFixture builds a small package containing a resource, an object type and a function
// (each with input/output properties) so that selfRef behaviour can be exercised against every
// kind of `IsWithin` pairing.
func buildSelfRefFixture(t *testing.T) (*schema.Package, *schema.Resource, *schema.ObjectType, *schema.Function) {
	t.Helper()
	pkg, err := schema.ImportSpec(schema.PackageSpec{
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

	var widget *schema.Resource
	for _, r := range pkg.Resources {
		if r.Token == "demo:mod/widget:Widget" {
			widget = r
		}
	}
	require.NotNil(t, widget)
	var settings *schema.ObjectType
	for _, typ := range pkg.Types {
		if obj, ok := typ.(*schema.ObjectType); ok && obj.Token == "demo:mod/Settings:Settings" {
			settings = obj
		}
	}
	require.NotNil(t, settings)
	var getWidget *schema.Function
	for _, f := range pkg.Functions {
		if f.Token == "demo:mod:getWidget" {
			getWidget = f
		}
	}
	require.NotNil(t, getWidget)
	return pkg, widget, settings, getWidget
}

func TestResolveDocRef(t *testing.T) {
	t.Parallel()

	t.Run("basic refs", func(t *testing.T) {
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
				// "s3" is the module; Go qualifies cross-module refs with the package alias.
				expected: "See s3.Bucket.",
			},
			{
				name:        "resource input property",
				description: "See {{% ref #/resources/aws:s3%2Fbucket:Bucket/inputProperties/corsRules %}}.",
				expected:    "See s3.BucketArgs.CorsRules.",
			},
			{
				name:        "type",
				description: "See {{% ref #/types/aws:s3%2FBucketCorsRule:BucketCorsRule %}}.",
				expected:    "See s3.BucketCorsRule.",
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

	t.Run("function/resource rename conflict", func(t *testing.T) {
		t.Parallel()

		// `pkg:conflict:newResource` collides with the generated `New<Resource>` constructor
		// for `pkg:conflict:Resource`, so Go codegen renames the function to `CreateResource`.
		// DocRefResolver must honour that rename.
		pkg, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "pkg",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"pkg:conflict:Resource": {},
			},
			Functions: map[string]schema.FunctionSpec{
				"pkg:conflict:newResource": {},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)

		d := DocLanguageHelper{}
		got := renderWithResolver(t, d, pkg.Reference(), schema.DocRef{},
			"See {{% ref #/functions/pkg:conflict:newResource %}}.")
		// The function name is derived directly from the token (no rename lookup in the
		// function branch of the resolver today), so this asserts the current behaviour and
		// will alert us if rename handling is extended to functions.
		assert.Contains(t, got, "NewResource")
	})

	t.Run("duplicate type/resource token suffix", func(t *testing.T) {
		t.Parallel()

		// An object type that shares a token (case-insensitively) with a resource gets a
		// `Type` suffix during Go codegen. Doc refs must resolve to the suffixed name.
		pkg, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "dup",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"dup:idx:Thing": {},
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
			"See {{% ref #/types/dup:idx:Thing %}}.")
		assert.Contains(t, got, "ThingType")
	})

	t.Run("selfRef", func(t *testing.T) {
		t.Parallel()
		fixture, widget, settings, getWidget := buildSelfRefFixture(t)
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
				description: "See {{% ref #/resources/demo:mod%2Fwidget:Widget/properties/color %}}.",
				expected:    "See Color.",
			},
			{
				name:        "resource self-input-property unqualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/resources/demo:mod%2Fwidget:Widget/inputProperties/size %}}.",
				expected:    "See Size.",
			},
			{
				name:        "resource ref to other entity stays qualified",
				selfRef:     schema.DocRefForResource(widget),
				description: "See {{% ref #/types/demo:mod%2FSettings:Settings/properties/verbose %}}.",
				expected:    "See mod.Settings.Verbose.",
			},
			{
				name:        "type self-property unqualified",
				selfRef:     schema.DocRefForType(settings),
				description: "See {{% ref #/types/demo:mod%2FSettings:Settings/properties/verbose %}}.",
				expected:    "See Verbose.",
			},
			{
				name:        "function self-input-property unqualified",
				selfRef:     schema.DocRefForFunction(getWidget),
				description: "See {{% ref #/functions/demo:mod:getWidget/inputs/properties/name %}}.",
				expected:    "See Name.",
			},
			{
				name:        "function self-output-property unqualified",
				selfRef:     schema.DocRefForFunction(getWidget),
				description: "See {{% ref #/functions/demo:mod:getWidget/outputs/properties/id %}}.",
				expected:    "See Id.",
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

	t.Run("ModuleToPackage override applied", func(t *testing.T) {
		t.Parallel()

		// Verify that GoPackageInfo.ModuleToPackage remappings (which change the import
		// alias used in cross-module refs) flow through to DocRefResolver.
		pkg, err := schema.ImportSpec(schema.PackageSpec{
			Name:    "pkg",
			Version: "0.0.1",
			Meta:    &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Resources: map[string]schema.ResourceSpec{
				"pkg:original/widget:Widget": {},
			},
		}, nil, schema.ValidationOptions{AllowDanglingReferences: true})
		require.NoError(t, err)
		pkg.Language["go"] = GoPackageInfo{
			ModuleToPackage: map[string]string{"original": "renamed"},
		}

		d := DocLanguageHelper{}
		got := renderWithResolver(t, d, pkg.Reference(), schema.DocRef{},
			"See {{% ref #/resources/pkg:original%2Fwidget:Widget %}}.")
		assert.Equal(t, "See renamed.Widget.", got)
	})
}

// Calling GetFunctionName may return the wrong result when
// [DocLanguageHelper.GeneratePackagesMap] is not called, but it shouldn't panic.
func TestGetFunctionNameWithoutPackageMapDoesNotPanic(t *testing.T) {
	t.Parallel()

	pkg, err := schema.ImportSpec(schema.PackageSpec{
		Name:    "pkg",
		Version: "0.0.1",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Functions: map[string]schema.FunctionSpec{
			"pkg:index:getSomeFunction": {},
		},
	}, nil, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err)
	d := DocLanguageHelper{}

	assert.Equal(t, "GetSomeFunction", d.GetFunctionName(pkg.Functions[0]))
}
