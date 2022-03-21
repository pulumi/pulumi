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
	"sort"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func readSchemaFile(file string) (pkgSpec PackageSpec) {
	// Read in, decode, and import the schema.
	schemaBytes, err := ioutil.ReadFile(filepath.Join("..", "testing", "test", "testdata", file))
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
		panic(err)
	}

	return pkgSpec
}

func TestImportSpec(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	for _, tt := range enumTests {
		tt := tt
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Read in, decode, and import the schema.
			schemaBytes, err := ioutil.ReadFile(
				filepath.Join("..", "testing", "test", "testdata", tt.schemaFile))
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
	t.Parallel()

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
		{
			name: "hyphenatedUrlPath",
			ref:  "/azure-native/v1.22.0/schema.json#/resources/azure-native:web:WebApp",
			want: typeSpecRef{
				URL:     toURL("/azure-native/v1.22.0/schema.json#/resources/azure-native:web:WebApp"),
				Package: "azure-native",
				Version: toVersionPtr("1.22.0"),
				Kind:    "resources",
				Token:   "azure-native:web:WebApp",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, diags := typs.parseTypeSpecRef("ref", tt.ref)
			if diags.HasErrors() != tt.wantErr {
				t.Errorf("parseTypeSpecRef() diags = %v, wantErr %v", diags, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTypeSpecRef() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMethods(t *testing.T) {
	t.Parallel()

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
			expectedError: "unknown function xyz:index:Foo/bar",
		},
		{
			filename:      "bad-methods-2.json",
			expectedError: "function xyz:index:Foo/bar is already a method",
		},
		{
			filename:      "bad-methods-3.json",
			expectedError: "invalid function token format xyz:index:Foo",
		},
		{
			filename:      "bad-methods-4.json",
			expectedError: "invalid function token format xyz:index:Baz/bar",
		},
		{
			filename:      "bad-methods-5.json",
			expectedError: "function xyz:index:Foo/bar has no __self__ parameter",
		},
		{
			filename:      "bad-methods-6.json",
			expectedError: "xyz:index:Foo already has a property named bar",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

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

// TestIsOverlay tests that the IsOverlay field is set correctly for resources, types, and functions. Does not test
// codegen.
func TestIsOverlay(t *testing.T) {
	t.Parallel()

	t.Run("overlay", func(t *testing.T) {
		t.Parallel()

		pkgSpec := readSchemaFile(filepath.Join("schema", "overlay.json"))

		pkg, err := ImportSpec(pkgSpec, nil)
		if err != nil {
			t.Error(err)
		}
		for _, v := range pkg.Resources {
			if strings.Contains(v.Token, "Overlay") {
				assert.Truef(t, v.IsOverlay, "resource %q", v.Token)
			} else {
				assert.Falsef(t, v.IsOverlay, "resource %q", v.Token)
			}
		}
		for _, v := range pkg.Types {
			switch v := v.(type) {
			case *ObjectType:
				if strings.Contains(v.Token, "Overlay") {
					assert.Truef(t, v.IsOverlay, "object type %q", v.Token)
				} else {
					assert.Falsef(t, v.IsOverlay, "object type %q", v.Token)
				}
			}
		}
		for _, v := range pkg.Functions {
			if strings.Contains(v.Token, "Overlay") {
				assert.Truef(t, v.IsOverlay, "function %q", v.Token)
			} else {
				assert.Falsef(t, v.IsOverlay, "function %q", v.Token)
			}
		}
	})
}

// Tests that the method ReplaceOnChanges works as expected. Does not test
// codegen.
func TestReplaceOnChanges(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		filePath string
		resource string
		result   []string
		errors   []string
	}{
		{
			name:     "Simple case",
			filePath: "replace-on-changes-1.json",
			resource: "example::Dog",
			result:   []string{"bone"},
		},
		{
			name:     "No replaceOnChanges",
			filePath: "replace-on-changes-2.json",
			resource: "example::Dog",
		},
		{
			name:     "Mutually Recursive",
			filePath: "replace-on-changes-3.json",
			resource: "example::Pets",
			result: []string{
				"cat.fish",
				"dog.bone",
				"dog.cat.fish",
				"cat.dog.bone"},
			errors: []string{
				"Failed to genereate full `ReplaceOnChanges`: Found recursive object \"cat\"",
				"Failed to genereate full `ReplaceOnChanges`: Found recursive object \"dog\""},
		},
		{
			name:     "Singularly Recursive",
			filePath: "replace-on-changes-4.json",
			resource: "example::Pets",
			result:   []string{"dog.bone"},
			errors:   []string{"Failed to genereate full `ReplaceOnChanges`: Found recursive object \"dog\""},
		},
		{
			name:     "Drill Correctly",
			filePath: "replace-on-changes-5.json",
			resource: "example::Pets",
			result:   []string{"foes.*.color", "friends[*].color", "name", "toy.color"},
		},
		{
			name:     "No replace on changes and recursive",
			filePath: "replace-on-changes-6.json",
			resource: "example::Child",
			result:   []string{},
			errors:   []string{},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// We sort each result before comparison. We don't enforce that the
			// results have the same order, just the same content.
			sort.Strings(tt.result)
			sort.Strings(tt.errors)
			pkgSpec := readSchemaFile(
				filepath.Join("schema", tt.filePath))
			pkg, err := ImportSpec(pkgSpec, nil)
			assert.NoError(t, err, "Import should be successful")
			resource, found := pkg.GetResource(tt.resource)
			assert.True(t, found, "The resource should exist")
			replaceOnChanges, errListErrors := resource.ReplaceOnChanges()
			errList := make([]string, len(errListErrors))
			for i, e := range errListErrors {
				errList[i] = e.Error()
			}
			actualResult := PropertyListJoinToString(replaceOnChanges,
				func(x string) string { return x })
			sort.Strings(actualResult)
			if tt.result != nil || len(actualResult) > 0 {
				assert.Equal(t, tt.result, actualResult,
					"Get the correct result")
			}
			if tt.errors != nil || len(errList) > 0 {
				assert.Equal(t, tt.errors, errList,
					"Get correct error messages")
			}
		})
	}
}

func TestValidateTypeToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		input         string
		expectError   bool
		allowedExtras []string
	}{
		{
			name:  "valid",
			input: "example::typename",
		},
		{
			name:        "invalid",
			input:       "xyz::typename",
			expectError: true,
		},
		{
			name:  "valid-has-subsection",
			input: "example:index:typename",
		},
		{
			name:        "invalid-has-subsection",
			input:       "not:index:typename",
			expectError: true,
		},
		{
			name:          "allowed-extras-valid",
			input:         "other:index:typename",
			allowedExtras: []string{"other"},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			spec := &PackageSpec{Name: "example"}
			allowed := map[string]bool{"example": true}
			for _, e := range c.allowedExtras {
				allowed[e] = true
			}
			errors := spec.validateTypeToken(allowed, "type", c.input)
			if c.expectError {
				assert.True(t, errors.HasErrors())
			} else {
				assert.False(t, errors.HasErrors())
			}
		})
	}
}
