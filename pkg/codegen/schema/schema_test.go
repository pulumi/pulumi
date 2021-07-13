// Copyright 2016-2020, Pulumi Corporation.
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

// nolint: lll
package schema

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func readSchemaFile(file string) (pkgSpec PackageSpec) {
	// Read in, decode, and import the schema.
	schemaBytes, err := ioutil.ReadFile(filepath.Join("..", "internal", "test", "testdata", file))
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
		panic(err)
	}

	return pkgSpec
}

func TestImportSpec(t *testing.T) {
	// Read in, decode, and import the schema.
	pkgSpec := readSchemaFile("kubernetes.json")

	pkg, err := ImportSpec(pkgSpec, nil)
	if err != nil {
		t.Errorf("ImportSpec() error = %v", err)
	}

	for _, r := range pkg.Resources {
		assert.NotNil(t, r.Package, "expected resource %s to have an associated Package", r.Token)
	}
}

var enumTests = []struct {
	filename    string
	shouldError bool
	expected    *EnumType
}{
	{"bad-enum-1.json", true, nil},
	{"bad-enum-2.json", true, nil},
	{"bad-enum-3.json", true, nil},
	{"bad-enum-4.json", true, nil},
	{"good-enum-1.json", false, &EnumType{
		Token:       "fake-provider:module1:Color",
		ElementType: stringType,
		Elements: []*Enum{
			{Value: "Red"},
			{Value: "Orange"},
			{Value: "Yellow"},
			{Value: "Green"},
		},
	}},
	{"good-enum-2.json", false, &EnumType{
		Token:       "fake-provider:module1:Number",
		ElementType: intType,
		Elements: []*Enum{
			{Value: int32(1), Name: "One"},
			{Value: int32(2), Name: "Two"},
			{Value: int32(3), Name: "Three"},
			{Value: int32(6), Name: "Six"},
		},
	}},
	{"good-enum-3.json", false, &EnumType{
		Token:       "fake-provider:module1:Boolean",
		ElementType: boolType,
		Elements: []*Enum{
			{Value: true, Name: "One"},
			{Value: false, Name: "Zero"},
		},
	}},
	{"good-enum-4.json", false, &EnumType{
		Token:       "fake-provider:module1:Number2",
		ElementType: numberType,
		Comment:     "what a great description",
		Elements: []*Enum{
			{Value: float64(1), Comment: "one", Name: "One"},
			{Value: float64(2), Comment: "two", Name: "Two"},
			{Value: 3.4, Comment: "3.4", Name: "ThreePointFour"},
			{Value: float64(6), Comment: "six", Name: "Six"},
		},
	}},
}

func TestEnums(t *testing.T) {
	for _, tt := range enumTests {
		t.Run(tt.filename, func(t *testing.T) {
			pkgSpec := readSchemaFile(filepath.Join("schema", tt.filename))

			pkg, err := ImportSpec(pkgSpec, nil)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				if err != nil {
					t.Error(err)
				}
				result := pkg.Types[0]
				tt.expected.Package = pkg
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestImportResourceRef(t *testing.T) {
	tests := []struct {
		name       string
		schemaFile string
		wantErr    bool
		validator  func(pkg *Package)
	}{
		{
			"simple",
			"simple-resource-schema/schema.json",
			false,
			func(pkg *Package) {
				for _, r := range pkg.Resources {
					if r.Token == "example::OtherResource" {
						for _, p := range r.Properties {
							if p.Name == "foo" {
								assert.IsType(t, &ResourceType{}, plainType(p.Type))
							}
						}
					}
				}
			},
		},
		{
			"external-ref",
			"external-resource-schema/schema.json",
			false,
			func(pkg *Package) {
				typ, ok := pkg.GetType("example::Pet")
				assert.True(t, ok)
				pet, ok := typ.(*ObjectType)
				assert.True(t, ok)
				name, ok := pet.Property("name")
				assert.True(t, ok)
				assert.IsType(t, &ResourceType{}, plainType(name.Type))
				resource := plainType(name.Type).(*ResourceType)
				assert.NotNil(t, resource.Resource)

				for _, r := range pkg.Resources {
					switch r.Token {
					case "example::Cat":
						for _, p := range r.Properties {
							if p.Name == "name" {
								assert.IsType(t, stringType, plainType(p.Type))
							}
						}
					case "example::Workload":
						for _, p := range r.Properties {
							if p.Name == "pod" {
								assert.IsType(t, &ObjectType{}, plainType(p.Type))

								obj := plainType(p.Type).(*ObjectType)
								assert.NotNil(t, obj.Properties)
							}
						}
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read in, decode, and import the schema.
			schemaBytes, err := ioutil.ReadFile(
				filepath.Join("..", "internal", "test", "testdata", tt.schemaFile))
			assert.NoError(t, err)

			var pkgSpec PackageSpec
			err = json.Unmarshal(schemaBytes, &pkgSpec)
			assert.NoError(t, err)

			pkg, err := ImportSpec(pkgSpec, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ImportSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			tt.validator(pkg)
		})
	}
}

func Test_parseTypeSpecRef(t *testing.T) {
	toVersionPtr := func(version string) *semver.Version { v := semver.MustParse(version); return &v }
	toURL := func(rawurl string) *url.URL {
		parsed, err := url.Parse(rawurl)
		assert.NoError(t, err, "failed to parse ref")

		return parsed
	}

	typs := &types{
		pkg: &Package{
			Name:    "test",
			Version: toVersionPtr("1.2.3"),
		},
	}

	tests := []struct {
		name    string
		ref     string
		want    typeSpecRef
		wantErr bool
	}{
		{
			name: "resourceRef",
			ref:  "#/resources/example::Resource",
			want: typeSpecRef{
				URL:     toURL("#/resources/example::Resource"),
				Package: "test",
				Version: toVersionPtr("1.2.3"),
				Kind:    "resources",
				Token:   "example::Resource",
			},
		},
		{
			name: "typeRef",
			ref:  "#/types/kubernetes:admissionregistration.k8s.io%2fv1:WebhookClientConfig",
			want: typeSpecRef{
				URL:     toURL("#/types/kubernetes:admissionregistration.k8s.io%2fv1:WebhookClientConfig"),
				Package: "test",
				Version: toVersionPtr("1.2.3"),
				Kind:    "types",
				Token:   "kubernetes:admissionregistration.k8s.io/v1:WebhookClientConfig",
			},
		},
		{
			name: "providerRef",
			ref:  "#/provider",
			want: typeSpecRef{
				URL:     toURL("#/provider"),
				Package: "test",
				Version: toVersionPtr("1.2.3"),
				Kind:    "provider",
				Token:   "pulumi:providers:test",
			},
		},
		{
			name: "externalResourceRef",
			ref:  "/random/v2.3.1/schema.json#/resources/random:index%2frandomPet:RandomPet",
			want: typeSpecRef{
				URL:     toURL("/random/v2.3.1/schema.json#/resources/random:index%2frandomPet:RandomPet"),
				Package: "random",
				Version: toVersionPtr("2.3.1"),
				Kind:    "resources",
				Token:   "random:index/randomPet:RandomPet",
			},
		},
		{
			name:    "invalid externalResourceRef",
			ref:     "/random/schema.json#/resources/random:index%2frandomPet:RandomPet",
			wantErr: true,
		},
		{
			name: "externalTypeRef",
			ref:  "/kubernetes/v2.6.3/schema.json#/types/kubernetes:admissionregistration.k8s.io%2Fv1:WebhookClientConfig",
			want: typeSpecRef{
				URL:     toURL("/kubernetes/v2.6.3/schema.json#/types/kubernetes:admissionregistration.k8s.io%2Fv1:WebhookClientConfig"),
				Package: "kubernetes",
				Version: toVersionPtr("2.6.3"),
				Kind:    "types",
				Token:   "kubernetes:admissionregistration.k8s.io/v1:WebhookClientConfig",
			},
		},
		{
			name: "externalHostResourceRef",
			ref:  "https://example.com/random/v2.3.1/schema.json#/resources/random:index%2FrandomPet:RandomPet",
			want: typeSpecRef{
				URL:     toURL("https://example.com/random/v2.3.1/schema.json#/resources/random:index%2FrandomPet:RandomPet"),
				Package: "random",
				Version: toVersionPtr("2.3.1"),
				Kind:    "resources",
				Token:   "random:index/randomPet:RandomPet",
			},
		},
		{
			name: "externalProviderRef",
			ref:  "/kubernetes/v2.6.3/schema.json#/provider",
			want: typeSpecRef{
				URL:     toURL("/kubernetes/v2.6.3/schema.json#/provider"),
				Package: "kubernetes",
				Version: toVersionPtr("2.6.3"),
				Kind:    "provider",
				Token:   "pulumi:providers:kubernetes",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typs.parseTypeSpecRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTypeSpecRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTypeSpecRef() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMethods(t *testing.T) {
	var tests = []struct {
		filename      string
		validator     func(pkg *Package)
		expectedError string
	}{
		{
			filename: "good-methods-1.json",
			validator: func(pkg *Package) {
				assert.Len(t, pkg.Resources, 1)
				assert.Len(t, pkg.Resources[0].Methods, 1)

				assert.NotNil(t, pkg.Resources[0].Methods[0].Function.Inputs)
				assert.Len(t, pkg.Resources[0].Methods[0].Function.Inputs.Properties, 1)
				inputs := pkg.Resources[0].Methods[0].Function.Inputs.Properties
				assert.Equal(t, "__self__", inputs[0].Name)
				assert.Equal(t, &ResourceType{
					Token:    pkg.Resources[0].Token,
					Resource: pkg.Resources[0],
				}, inputs[0].Type)

				assert.NotNil(t, pkg.Resources[0].Methods[0].Function.Outputs)
				assert.Len(t, pkg.Resources[0].Methods[0].Function.Outputs.Properties, 1)
				outputs := pkg.Resources[0].Methods[0].Function.Outputs.Properties
				assert.Equal(t, "someValue", outputs[0].Name)
				assert.Equal(t, StringType, outputs[0].Type)

				assert.Len(t, pkg.Functions, 1)
				assert.True(t, pkg.Functions[0].IsMethod)
				assert.Same(t, pkg.Resources[0].Methods[0].Function, pkg.Functions[0])
			},
		},
		{
			filename:      "bad-methods-1.json",
			expectedError: "unknown function xyz:index:Foo/bar for method bar",
		},
		{
			filename:      "bad-methods-2.json",
			expectedError: "function xyz:index:Foo/bar for method baz is already a method",
		},
		{
			filename:      "bad-methods-3.json",
			expectedError: "invalid function token format xyz:index:Foo for method bar",
		},
		{
			filename:      "bad-methods-4.json",
			expectedError: "invalid function token format xyz:index:Baz/bar for method bar",
		},
		{
			filename:      "bad-methods-5.json",
			expectedError: "function xyz:index:Foo/bar for method bar is missing __self__ parameter",
		},
		{
			filename:      "bad-methods-6.json",
			expectedError: "property and method have the same name bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			pkgSpec := readSchemaFile(filepath.Join("schema", tt.filename))

			pkg, err := ImportSpec(pkgSpec, nil)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				if err != nil {
					t.Error(err)
				}
				tt.validator(pkg)
			}
		})
	}
}
