// Copyright 2016-2025, Pulumi Corporation.
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

// Integration level tests for docs across all languages with docs generation in this
// repository.
//
// To avoid circular dependencies, we will not attempt to test Java or Pulumi YAML here.
package codegen_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	dotnet_codegen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	golang_codegen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	nodejs_codegen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	python_codegen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type language int

const (
	nodejs language = iota
	python language = iota
	golang language = iota
	dotnet language = iota
)

func TestGetLanguageTypeString(t *testing.T) {
	t.Parallel()

	schema1 := bind(t, schema.PackageSpec{
		Name: "pkg",
		Types: map[string]schema.ComplexTypeSpec{
			"pkg:index:simpleType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
				},
			},
			"pkg:module:anotherType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
				},
			},
			"pkg:module:anEnum": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "string",
				},
				Enum: []schema.EnumValueSpec{{
					Name:  "Value1",
					Value: "value1",
				}},
			},
		},
	})

	schemaWithOverrides := bind(t, schema.PackageSpec{
		Name: "pkg",
		Types: map[string]schema.ComplexTypeSpec{
			"pkg:shouldoverride:simpleType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
				},
			},
		},
		Language: map[string]schema.RawMessage{
			"go": marshalIntoRaw(t, golang_codegen.GoPackageInfo{
				ModuleToPackage: map[string]string{
					"shouldoverride": "overridden",
				},
			}),
			"csharp": marshalIntoRaw(t, dotnet_codegen.CSharpPackageInfo{
				Namespaces: map[string]string{
					"shouldoverride": "Overridden",
				},
			}),
			"nodejs": marshalIntoRaw(t, nodejs_codegen.NodePackageInfo{
				ModuleToPackage: map[string]string{
					"shouldoverride": "overridden",
				},
			}),
			"python": marshalIntoRaw(t, python_codegen.PackageInfo{
				ModuleNameOverrides: map[string]string{
					"shouldoverride": "overridden",
				},
			}),
		},
	})

	tests := []struct {
		name   string
		schema schema.PackageReference

		// Arguments

		module string
		typ    schema.Type
		input  *bool // if nil, assert on both inputs and outputs

		expected map[language]string
	}{
		{
			name:   "primitive-string",
			schema: schema.DefaultPulumiPackage.Reference(),
			typ:    schema.StringType,

			expected: map[language]string{
				golang: "string",
				nodejs: "string",
				python: "str",
				dotnet: "string",
			},
		},
		{
			name:   "map-of-primitive",
			schema: schema.DefaultPulumiPackage.Reference(),
			typ:    &schema.MapType{ElementType: schema.NumberType},

			expected: map[language]string{
				golang: "map[string]float64",
				nodejs: "{[key: string]: number}",
				python: "Mapping[str, float]",
				dotnet: "Dictionary<string, double>",
			},
		},
		{
			name:   "array-of-primitive",
			schema: schema.DefaultPulumiPackage.Reference(),
			typ:    &schema.ArrayType{ElementType: schema.BoolType},

			expected: map[language]string{
				golang: "[]bool",
				nodejs: "boolean[]",
				python: "Sequence[bool]",
				dotnet: "List<bool>",
			},
		},
		{
			name:   "object",
			schema: schema1,
			typ:    mustToken(t, schema1.Types().Get, "pkg:index:simpleType"),
			input:  ptr(true),

			expected: map[language]string{
				golang: "SimpleType",
				nodejs: "SimpleType",
				python: "SimpleType",
				dotnet: "Pulumi.Pkg.Inputs.SimpleType",
			},
		},
		{
			name:   "object",
			schema: schema1,
			typ:    mustToken(t, schema1.Types().Get, "pkg:index:simpleType"),
			input:  ptr(false),

			expected: map[language]string{
				golang: "SimpleType",
				nodejs: "SimpleType",
				python: "SimpleType",
				dotnet: "Pulumi.Pkg.Outputs.SimpleType",
			},
		},
		{
			name:   "map-of-object",
			schema: schema1,
			typ:    &schema.MapType{ElementType: mustToken(t, schema1.Types().Get, "pkg:index:simpleType")},
			input:  ptr(false),

			expected: map[language]string{
				golang: "map[string]SimpleType",
				nodejs: "{[key: string]: SimpleType}",
				python: "Mapping[str, SimpleType]",
				dotnet: "Dictionary<string, Pulumi.Pkg.Outputs.SimpleType>",
			},
		},
		{
			name:   "module-object",
			schema: schema1,
			typ:    mustToken(t, schema1.Types().Get, "pkg:module:anotherType"),
			input:  ptr(true),

			expected: map[language]string{
				golang: "module.AnotherType",
				nodejs: "module.AnotherType",
				python: "_module.AnotherType",
				dotnet: "Pulumi.Pkg.Module.Inputs.AnotherType",
			},
		},
		{
			name:   "module-object-from-module",
			schema: schema1,
			typ:    mustToken(t, schema1.Types().Get, "pkg:module:anotherType"),
			input:  ptr(true),
			module: "module",

			expected: map[language]string{
				golang: "AnotherType",
				nodejs: "module.AnotherType",
				python: "AnotherType",
				dotnet: "Pulumi.Pkg.Module.Inputs.AnotherType",
			},
		},
		{
			name:   "enum-in-module",
			schema: schema1,
			typ:    mustToken(t, schema1.Types().Get, "pkg:module:anEnum"),
			module: "module",
			expected: map[language]string{
				golang: "AnEnum",
				nodejs: "module.AnEnum",
				python: "AnEnum",
				dotnet: "Pulumi.Pkg.Module.AnEnum",
			},
		},
		{
			name:   "overridden-names-in-module",
			schema: schemaWithOverrides,
			typ:    mustToken(t, schemaWithOverrides.Types().Get, "pkg:shouldoverride:simpleType"),
			module: schemaWithOverrides.TokenToModule("pkg:shouldoverride:simpleType"),
			input:  ptr(true),
			expected: map[language]string{
				golang: "SimpleType",
				nodejs: "overridden.SimpleType",
				python: "SimpleType",
				dotnet: "Pulumi.Pkg.Overridden.Inputs.SimpleType",
			},
		},
		{
			name:   "overridden-names",
			schema: schemaWithOverrides,
			typ:    mustToken(t, schemaWithOverrides.Types().Get, "pkg:shouldoverride:simpleType"),
			input:  ptr(false),
			expected: map[language]string{
				golang: "overridden.SimpleType",
				nodejs: "overridden.SimpleType",
				python: "_overridden.SimpleType",
				dotnet: "Pulumi.Pkg.Overridden.Outputs.SimpleType",
			},
		},
		{
			name:   "optionals",
			schema: schema.DefaultPulumiPackage.Reference(),
			typ:    &schema.OptionalType{ElementType: schema.StringType},
			expected: map[language]string{
				golang: "*string",
				nodejs: "string",
				python: "Optional[str]",
				dotnet: "string?",
			},
		},
	}

	// Code generation is not safe to parallelize since import binding mutates the
	// [schema.Package].
	for _, tt := range tests { //nolint:paralleltest
		t.Run(tt.name, func(t *testing.T) {
			require.NotEmpty(t, tt.expected, "Must test at least one language")
			for lang, expected := range tt.expected {
				testDocsGenHelper(t, lang, tt.schema, func(t *testing.T, helper codegen.DocLanguageHelper) {
					if tt.input == nil || *tt.input {
						t.Run("input", func(t *testing.T) { //nolint:paralleltest // golangci-lint v2 upgrade
							actual := helper.GetTypeName(tt.schema, tt.typ, true, tt.module)
							assert.Equal(t, expected, actual)
						})
					}
					if tt.input == nil || !*tt.input {
						t.Run("output", func(t *testing.T) { //nolint:paralleltest // golangci-lint v2 upgrade
							actual := helper.GetTypeName(tt.schema, tt.typ, false, tt.module)
							assert.Equal(t, expected, actual)
						})
					}
				})
			}
		})
	}
}

func TestGetMethodResultName(t *testing.T) {
	t.Parallel()

	schema1 := bind(t, schema.PackageSpec{
		Name: "example",
		Resources: map[string]schema.ResourceSpec{
			"example:index:Foo": {
				IsComponent: true,
				Methods: map[string]string{
					"getKubeconfig": "example:index:Foo/getKubeconfig",
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"example:index:Foo/getKubeconfig": {
				Inputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"__self__": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/resources/example:index:Foo",
							},
						},
						"profileName": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"roleArn": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"__self__"},
				},
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"kubeconfig": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"kubeconfig"},
				},
			},
		},
		Language: map[string]schema.RawMessage{
			"csharp": schema.RawMessage(`{"liftSingleValueMethodReturns": true}`),
			"go": schema.RawMessage(`{
			"importBasePath": "simple-methods-schema-single-value-returns/example",
			"liftSingleValueMethodReturns": true,
			"generateExtraInputTypes": true
		}`),
			"nodejs": schema.RawMessage(`{
			"devDependencies": {
				"@types/node": "ts4.3"
			},
			"liftSingleValueMethodReturns": true
		}`),
			"python": schema.RawMessage(`{"liftSingleValueMethodReturns": true}`),
		},
	})

	schema2 := bind(t, schema.PackageSpec{
		Name: "example",
		Resources: map[string]schema.ResourceSpec{
			"example:index:Foo": {
				IsComponent: true,
				Methods: map[string]string{
					"getKubeconfig": "example:index:Foo/getKubeconfig",
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"example:index:Foo/getKubeconfig": {
				Inputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"__self__": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/resources/example:index:Foo",
							},
						},
						"profileName": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"roleArn": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"__self__"},
				},
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"kubeconfig": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"kubeconfig"},
				},
			},
		},
	})

	tests := []struct {
		name   string
		schema schema.PackageReference

		// Arguments

		module   string
		resource *schema.Resource
		method   *schema.Method

		expected map[language]string
	}{
		{
			name:     "single-return-value",
			schema:   schema1,
			resource: mustToken(t, schema1.Resources().Get, "example:index:Foo"),
			method:   mustToken(t, schema1.Resources().Get, "example:index:Foo").Methods[0],
			expected: map[language]string{
				golang: "pulumi.StringOutput",
				nodejs: "string",
				python: "str",
				dotnet: "string",
			},
		},
		{
			name:     "object-return-value",
			schema:   schema2,
			resource: mustToken(t, schema2.Resources().Get, "example:index:Foo"),
			method:   mustToken(t, schema2.Resources().Get, "example:index:Foo").Methods[0],
			expected: map[language]string{
				golang: "FooGetKubeconfigResultOutput",
				nodejs: "Foo.GetKubeconfigResult",
				python: "Foo.Get_kubeconfigResult",
				dotnet: "Foo.GetKubeconfigResult",
			},
		},
	}

	// Code generation is not safe to parallelize since import binding mutates the
	// [schema.Package].
	for _, tt := range tests { //nolint:paralleltest
		t.Run(tt.name, func(t *testing.T) {
			require.NotEmpty(t, tt.expected, "Must test at least one language")
			for lang, expected := range tt.expected {
				testDocsGenHelper(t, lang, tt.schema, func(t *testing.T, helper codegen.DocLanguageHelper) {
					actual := helper.GetMethodResultName(tt.schema, tt.module, tt.resource, tt.method)
					assert.Equal(t, expected, actual)
				})
			}
		})
	}
}

func TestGetMethodResultName_NoImporter(t *testing.T) {
	t.Parallel()

	schemaSpec := schema.PackageSpec{
		Name: "example",
		Resources: map[string]schema.ResourceSpec{
			"example:index:Foo": {
				IsComponent: true,
				Methods: map[string]string{
					"getKubeconfig": "example:index:Foo/getKubeconfig",
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"example:index:Foo/getKubeconfig": {
				Inputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"__self__": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/resources/example:index:Foo",
							},
						},
						"profileName": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"roleArn": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"__self__"},
				},
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"kubeconfig": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"kubeconfig"},
				},
			},
		},
	}

	pkg, err := schema.ImportSpec(schemaSpec, nil, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err)

	expected := map[language]string{
		golang: "FooGetKubeconfigResultOutput",
		nodejs: "Foo.GetKubeconfigResult",
		python: "Foo.Get_kubeconfigResult",
		dotnet: "Foo.GetKubeconfigResult",
	}

	// Code generation is not safe to parallelize since import binding mutates the
	// [schema.Package].
	for lang, expected := range expected {
		testDocsGenHelper(t, lang, pkg.Reference(), func(t *testing.T, helper codegen.DocLanguageHelper) {
			actual := helper.GetMethodResultName(pkg.Reference(), "",
				mustToken(t, pkg.Reference().Resources().Get, "example:index:Foo"),
				mustToken(t, pkg.Reference().Resources().Get, "example:index:Foo").Methods[0],
			)
			assert.Equal(t, expected, actual)
		})
	}
}

func testDocsGenHelper(
	t *testing.T, language language, schema schema.PackageReference,
	f func(*testing.T, codegen.DocLanguageHelper),
) {
	var name string
	var helper func() codegen.DocLanguageHelper
	switch language {
	case nodejs:
		helper = mkHelper[nodejs_codegen.DocLanguageHelper]
		name = "nodejs"
	case python:
		helper = mkHelper[python_codegen.DocLanguageHelper]
		name = "python"
	case golang:
		helper = func() codegen.DocLanguageHelper {
			h := golang_codegen.DocLanguageHelper{}
			var info golang_codegen.GoPackageInfo
			if i, err := schema.Language("go"); err == nil && i != nil {
				info = i.(golang_codegen.GoPackageInfo)
			}
			h.GeneratePackagesMap(schema, "test", info)
			return h
		}
		name = "go"
	case dotnet:
		helper = mkHelper[dotnet_codegen.DocLanguageHelper]
		name = "dotnet"
	default:
		assert.Fail(t, "Unknown language %T", language)
	}

	t.Run(name, func(t *testing.T) { //nolint:paralleltest // golangci-lint v2 upgrade
		f(t, helper())
	})
}

func BenchmarkGetPropertyNames(b *testing.B) {
	schemaBytes, err := os.ReadFile("../../tests/testdata/codegen/azure-native-2.41.0.json")
	require.NoError(b, err)
	b.Run("full-bind", func(b *testing.B) {
		for range b.N {
			var spec schema.PackageSpec
			require.NoError(b, json.Unmarshal(schemaBytes, &spec))
			partial, err := schema.ImportSpec(spec, map[string]schema.Language{
				"nodejs": nodejs_codegen.Importer,
			}, schema.ValidationOptions{})
			require.NoError(b, err)

			res, ok := partial.GetResource("azure-native:eventgrid/v20220615:DomainTopicEventSubscription")
			require.True(b, ok)

			var helper nodejs_codegen.DocLanguageHelper
			for _, prop := range res.InputProperties {
				helper.GetTypeName(partial.Reference(), prop.Type, false, partial.TokenToModule(res.Token))
			}
		}
	})

	b.Run("partial-bind", func(b *testing.B) {
		for range b.N {
			var spec schema.PartialPackageSpec
			require.NoError(b, json.Unmarshal(schemaBytes, &spec))
			partial, err := schema.ImportPartialSpec(spec, map[string]schema.Language{
				"nodejs": nodejs_codegen.Importer,
			}, nil)
			require.NoError(b, err)

			res, ok, err := partial.Resources().Get("azure-native:eventgrid/v20220615:DomainTopicEventSubscription")
			require.NoError(b, err)
			require.True(b, ok)

			var helper nodejs_codegen.DocLanguageHelper
			for _, prop := range res.InputProperties {
				helper.GetTypeName(partial, prop.Type, false, partial.TokenToModule(res.Token))
			}
		}
	})
}

func bind(t *testing.T, spec schema.PackageSpec) schema.PackageReference {
	pkg, err := schema.ImportSpec(spec, map[string]schema.Language{
		"go":     golang_codegen.Importer,
		"nodejs": nodejs_codegen.Importer,
		"python": python_codegen.Importer,
		"csharp": dotnet_codegen.Importer,
	}, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NoError(t, err)
	return pkg.Reference()
}

func mustToken[T any](t *testing.T, get func(string) (T, bool, error), token string) T {
	v, ok, err := get(token)
	require.NoError(t, err)
	require.True(t, ok)
	return v
}

func ptr[T any](v T) *T { return &v }

func marshalIntoRaw(t *testing.T, v any) schema.RawMessage {
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return schema.RawMessage(b)
}

func mkHelper[T codegen.DocLanguageHelper]() codegen.DocLanguageHelper { var v T; return v }
