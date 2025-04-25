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
				python: "builtins.str", // TODO[https://github.com/pulumi/pulumi/issues/19272]
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
				python: "Mapping[str, builtins.float]", // TODO[https://github.com/pulumi/pulumi/issues/19272]
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
				python: "Sequence[builtins.bool]", // TODO[https://github.com/pulumi/pulumi/issues/19272]
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			def, err := tt.schema.Definition()
			require.NoError(t, err)
			require.NotEmpty(t, tt.expected, "Must test at least one language")
			for lang, expected := range tt.expected {
				var name string
				var helper codegen.DocLanguageHelper
				switch lang {
				case nodejs:
					helper = nodejs_codegen.DocLanguageHelper{}
					name = "nodejs"
				case python:
					helper = python_codegen.DocLanguageHelper{}
					name = "python"
				case golang:
					h := golang_codegen.DocLanguageHelper{}
					var info golang_codegen.GoPackageInfo
					if i, ok := def.Language["go"].(golang_codegen.GoPackageInfo); ok {
						info = i
					}
					h.GeneratePackagesMap(def, "test", info)
					helper = h
					name = "go"
				case dotnet:
					helper = dotnet_codegen.DocLanguageHelper{}
					name = "dotnet"
				default:
					assert.Fail(t, "Unknown language %T", lang)
				}

				t.Run(name, func(t *testing.T) {
					t.Parallel()

					if tt.input == nil || *tt.input {
						t.Run("input", func(t *testing.T) {
							actual := helper.GetLanguageTypeString(def, tt.module, tt.typ, true)
							assert.Equal(t, expected, actual)
						})
					}
					if tt.input == nil || !*tt.input {
						t.Run("output", func(t *testing.T) {
							actual := helper.GetLanguageTypeString(def, tt.module, tt.typ, false)
							assert.Equal(t, expected, actual)
						})
					}
				})
			}
		})
	}
}

func bind(t *testing.T, spec schema.PackageSpec) schema.PackageReference {
	pkg, err := schema.ImportSpec(spec, map[string]schema.Language{
		"go":     golang_codegen.Importer,
		"nodejs": nodejs_codegen.Importer,
		"python": python_codegen.Importer,
		"dotnet": dotnet_codegen.Importer,
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
