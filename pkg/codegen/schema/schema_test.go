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

//nolint:lll
package schema

import (
	"encoding/json"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func readSchemaFile(file string) (pkgSpec PackageSpec) {
	// Read in, decode, and import the schema.
	schemaBytes, err := os.ReadFile(filepath.Join("..", "testing", "test", "testdata", file))
	if err != nil {
		panic(err)
	}

	if strings.HasSuffix(file, ".json") {
		if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
			panic(err)
		}
	} else if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
		if err = yaml.Unmarshal(schemaBytes, &pkgSpec); err != nil {
			panic(err)
		}
	} else {
		panic("unknown schema file extension while parsing " + file)
	}

	return pkgSpec
}

func TestRoundtripRemoteTypeRef(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/13000
	t.Parallel()

	testdataPath := filepath.Join("..", "testing", "test", "testdata")
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := readSchemaFile("remoteref-1.0.0.json")
	pkg, diags, err := BindSpec(pkgSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
	newSpec, err := pkg.MarshalSpec()
	require.NoError(t, err)
	require.NotNil(t, newSpec)

	// Try and bind again
	_, diags, err = BindSpec(*newSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRoundtripLocalTypeRef(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/13671
	t.Parallel()

	testdataPath := filepath.Join("..", "testing", "test", "testdata")
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := readSchemaFile("localref-1.0.0.json")
	pkg, diags, err := BindSpec(pkgSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
	newSpec, err := pkg.MarshalSpec()
	require.NoError(t, err)
	require.NotNil(t, newSpec)

	// Try and bind again
	_, diags, err = BindSpec(*newSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRoundtripEnum(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/13921
	t.Parallel()

	assertEnum := func(t *testing.T, pkg *Package) {
		typ, ok := pkg.GetType("enum:index:Color")
		assert.True(t, ok)
		enum, ok := typ.(*EnumType)
		assert.True(t, ok)
		assert.Equal(t, "An enum representing a color", enum.Comment)
		assert.ElementsMatch(t, []*Enum{
			{Value: "red"},
			{Value: "green"},
			{Value: "blue"},
		}, enum.Elements)
	}

	testdataPath := filepath.Join("..", "testing", "test", "testdata")
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := readSchemaFile("enum-1.0.0.json")
	pkg, diags, err := BindSpec(pkgSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
	assertEnum(t, pkg)

	newSpec, err := pkg.MarshalSpec()
	require.NoError(t, err)
	require.NotNil(t, newSpec)

	// Try and bind again
	pkg, diags, err = BindSpec(*newSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
	assertEnum(t, pkg)
}

func TestRoundtripPlainProperties(t *testing.T) {
	t.Parallel()

	assertPlainnessFromType := func(t *testing.T, pkg *Package) {
		exampleType, ok := pkg.GetType("plain-properties:index:ExampleType")
		assert.True(t, ok)
		exampleObjectType, ok := exampleType.(*ObjectType)
		assert.True(t, ok)

		assert.Equal(t, 2, len(exampleObjectType.Properties))
		var exampleProperty *Property
		var nonPlainProperty *Property
		for _, p := range exampleObjectType.Properties {
			if p.Name == "exampleProperty" {
				exampleProperty = p
			}

			if p.Name == "nonPlainProperty" {
				nonPlainProperty = p
			}
		}

		assert.NotNil(t, exampleProperty)
		assert.NotNil(t, nonPlainProperty)

		assert.True(t, exampleProperty.Plain)
		assert.False(t, nonPlainProperty.Plain)
	}

	assertPlainnessFromResource := func(t *testing.T, pkg *Package) {
		exampleResource, ok := pkg.GetResource("plain-properties:index:ExampleResource")
		assert.True(t, ok)

		check := func(properties []*Property) {
			var exampleProperty *Property
			var nonPlainProperty *Property
			for _, p := range exampleResource.InputProperties {
				if p.Name == "exampleProperty" {
					exampleProperty = p
				}

				if p.Name == "nonPlainProperty" {
					nonPlainProperty = p
				}
			}

			// assert that the input property "exampleProperty" is plain
			assert.NotNil(t, exampleProperty)
			assert.True(t, exampleProperty.Plain)

			// assert that the output property is not plain
			assert.NotNil(t, nonPlainProperty)
			assert.False(t, nonPlainProperty.Plain)
		}

		check(exampleResource.InputProperties)
		check(exampleResource.Properties)
	}

	testdataPath := filepath.Join("..", "testing", "test", "testdata")
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := readSchemaFile("plain-properties-1.0.0.json")
	pkg, diags, err := BindSpec(pkgSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
	assertPlainnessFromType(t, pkg)
	assertPlainnessFromResource(t, pkg)

	newSpec, err := pkg.MarshalSpec()
	require.NoError(t, err)
	require.NotNil(t, newSpec)

	// Try and bind again
	pkg, diags, err = BindSpec(*newSpec, loader)
	require.NoError(t, err)
	assert.Empty(t, diags)
	assertPlainnessFromType(t, pkg)
	assertPlainnessFromResource(t, pkg)
}

func TestImportSpec(t *testing.T) {
	t.Parallel()

	// Read in, decode, and import the schema.
	pkgSpec := readSchemaFile("kubernetes-3.7.2.json")

	pkg, err := ImportSpec(pkgSpec, nil)
	if err != nil {
		t.Errorf("ImportSpec() error = %v", err)
	}

	for _, r := range pkg.Resources {
		assert.NotNil(t, r.PackageReference, "expected resource %s to have an associated Package", r.Token)
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

func TestUnmarshalYAMLFunctionSpec(t *testing.T) {
	t.Parallel()
	var functionSpec *FunctionSpec
	fnYaml := `
description: Test function
outputs:
  type: number`

	err := yaml.Unmarshal([]byte(fnYaml), &functionSpec)
	assert.Nil(t, err, "Unmarshalling should work")
	assert.Equal(t, "Test function", functionSpec.Description)
	assert.NotNil(t, functionSpec.ReturnType, "Return type is not nil")
	assert.NotNil(t, functionSpec.ReturnType.TypeSpec, "Return type is a type spec")
	assert.Equal(t, "number", functionSpec.ReturnType.TypeSpec.Type, "Return type is a number")
}

func TestUnmarshalJSONFunctionSpec(t *testing.T) {
	t.Parallel()
	var functionSpec *FunctionSpec
	fnJSON := `{"description":"Test function", "outputs": { "type": "number" } }`
	err := json.Unmarshal([]byte(fnJSON), &functionSpec)
	assert.Nil(t, err, "Unmarshalling should work")
	assert.Equal(t, "Test function", functionSpec.Description)
	assert.NotNil(t, functionSpec.ReturnType, "Return type is not nil")
	assert.NotNil(t, functionSpec.ReturnType.TypeSpec, "Return type is a type spec")
	assert.Equal(t, "number", functionSpec.ReturnType.TypeSpec.Type, "Return type is a number")
}

func TestMarshalJSONFunctionSpec(t *testing.T) {
	t.Parallel()
	functionSpec := &FunctionSpec{
		Description: "Test function",
		ReturnType: &ReturnTypeSpec{
			TypeSpec: &TypeSpec{Type: "number"},
		},
	}

	dataJSON, err := json.Marshal(functionSpec)
	data := string(dataJSON)
	expectedJSON := `{"description":"Test function","outputs":{"type":"number"}}`
	assert.Nil(t, err, "Unmarshalling should work")
	assert.Equal(t, expectedJSON, data)
}

func TestMarshalJSONFunctionSpecWithOutputs(t *testing.T) {
	t.Parallel()
	functionSpec := &FunctionSpec{
		Description: "Test function",
		Outputs: &ObjectTypeSpec{
			Type: "object",
			Properties: map[string]PropertySpec{
				"foo": {
					TypeSpec: TypeSpec{Type: "string"},
				},
			},
		},
	}

	dataJSON, err := json.Marshal(functionSpec)
	data := string(dataJSON)
	expectedJSON := `{"description":"Test function","outputs":{"properties":{"foo":{"type":"string"}},"type":"object"}}`
	assert.Nil(t, err, "Unmarshalling should work")
	assert.Equal(t, expectedJSON, data)
}

func TestMarshalYAMLFunctionSpec(t *testing.T) {
	t.Parallel()
	functionSpec := &FunctionSpec{
		Description: "Test function",
		ReturnType: &ReturnTypeSpec{
			TypeSpec: &TypeSpec{Type: "number"},
		},
	}

	dataYAML, err := yaml.Marshal(functionSpec)
	data := string(dataYAML)
	expectedYAML := `description: Test function
outputs:
    type: number
`

	assert.Nil(t, err, "Unmarshalling should work")
	assert.Equal(t, expectedYAML, data)
}

func TestInvalidTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		expected string
	}{
		{"bad-type-1.json", "invalid token 'fake-provider:index:provider' (provider is a reserved word for the root module)"},
		{"bad-type-2.json", "invalid token 'fake-provider::provider' (provider is a reserved word for the root module)"},
		{"bad-type-3.json", "invalid token 'fake-provider:noModulePart' (should have three parts)"},
		{"bad-type-4.json", "invalid token 'noParts' (should have three parts); "},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()

			pkgSpec := readSchemaFile(filepath.Join("schema", tt.filename))

			_, err := ImportSpec(pkgSpec, nil)
			assert.ErrorContains(t, err, tt.expected)
		})
	}
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
				tt.expected.PackageReference = pkg.Reference()
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

//nolint:paralleltest // needs to set plugin acquisition env var
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
	//nolint:paralleltest // needs to set plugin acquisition env var
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

			// Read in, decode, and import the schema.
			schemaBytes, err := os.ReadFile(
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

func TestUsingUrnInResourcePropertiesEmitsWarning(t *testing.T) {
	t.Parallel()
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:TestResource": {
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"urn": {
							TypeSpec: TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
			"test:index:TestComponent": {
				IsComponent: true,
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"urn": {
							TypeSpec: TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	pkg, diags, err := BindSpec(pkgSpec, loader)
	// No error as binding should work fine even with warnings
	assert.NoError(t, err)
	// assert that there are 2 warnings in the diagnostics because of using URN as a property
	assert.Len(t, diags, 2)
	for _, diag := range diags {
		assert.Equal(t, diag.Severity, hcl.DiagWarning)
		assert.Contains(t, diag.Summary, "urn is a reserved property name")
	}
	assert.NotNil(t, pkg)
}

func TestUsingIdInResourcePropertiesEmitsWarning(t *testing.T) {
	t.Parallel()
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:TestResource": {
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"id": {
							TypeSpec: TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	pkg, diags, err := BindSpec(pkgSpec, loader)
	// No error as binding should work fine even with warnings
	assert.NoError(t, err)
	assert.NotNil(t, pkg)
	// assert that there is 1 warning in the diagnostics because of using ID as a property
	assert.Len(t, diags, 1)
	assert.Equal(t, diags[0].Severity, hcl.DiagWarning)
	assert.Contains(t, diags[0].Summary, "id is a reserved property name")
}

func TestUsingIdInComponentResourcePropertiesEmitsNoWarning(t *testing.T) {
	t.Parallel()
	loader := NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]ResourceSpec{
			"test:index:TestComponent": {
				IsComponent: true,
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"id": {
							TypeSpec: TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	pkg, diags, err := BindSpec(pkgSpec, loader)
	assert.NoError(t, err)
	assert.Empty(t, diags)
	assert.NotNil(t, pkg)
}

func TestMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
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

				var objectReturnType *ObjectType
				if objectType, ok := pkg.Resources[0].Methods[0].Function.ReturnType.(*ObjectType); ok && objectType != nil {
					objectReturnType = objectType
				}

				assert.NotNil(t, objectReturnType)
				assert.Len(t, objectReturnType.Properties, 1)
				outputs := objectReturnType.Properties
				assert.Equal(t, "someValue", outputs[0].Name)
				assert.Equal(t, StringType, outputs[0].Type)

				assert.Len(t, pkg.Functions, 1)
				assert.True(t, pkg.Functions[0].IsMethod)
				assert.Same(t, pkg.Resources[0].Methods[0].Function, pkg.Functions[0])
			},
		},
		{
			filename: "good-simplified-methods.json",
			validator: func(pkg *Package) {
				assert.Len(t, pkg.Functions, 1)
				assert.NotNil(t, pkg.Functions[0].ReturnType, "There should be a return type")
				assert.Equal(t, pkg.Functions[0].ReturnType, NumberType)
			},
		},
		{
			filename: "good-simplified-methods.yml",
			validator: func(pkg *Package) {
				assert.Len(t, pkg.Functions, 1)
				assert.NotNil(t, pkg.Functions[0].ReturnType, "There should be a return type")
				assert.Equal(t, pkg.Functions[0].ReturnType, NumberType)
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
				assert.ErrorContains(t, err, tt.expectedError)
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

func TestBindingOutputsPopulatesReturnType(t *testing.T) {
	t.Parallel()

	// Test that using Outputs in PackageSpec correctly populates the return type of the function.
	pkgSpec := PackageSpec{
		Name:    "xyz",
		Version: "0.0.1",
		Functions: map[string]FunctionSpec{
			"xyz:index:abs": {
				MultiArgumentInputs: []string{"value"},
				Inputs: &ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"value": {
							TypeSpec: TypeSpec{
								Type: "number",
							},
						},
					},
				},
				Outputs: &ObjectTypeSpec{
					Required: []string{"result"},
					Properties: map[string]PropertySpec{
						"result": {
							TypeSpec: TypeSpec{
								Type: "number",
							},
						},
					},
				},
			},
		},
	}

	pkg, err := ImportSpec(pkgSpec, nil)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, pkg.Functions[0].ReturnType)
	objectType, ok := pkg.Functions[0].ReturnType.(*ObjectType)
	assert.True(t, ok)
	assert.Equal(t, NumberType, objectType.Properties[0].Type)
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
				"cat.dog.bone",
			},
			errors: []string{
				"Failed to genereate full `ReplaceOnChanges`: Found recursive object \"cat\"",
				"Failed to genereate full `ReplaceOnChanges`: Found recursive object \"dog\"",
			},
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

func TestTypeString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  Type
		output string
	}{
		{
			input: &UnionType{
				ElementTypes: []Type{
					StringType,
					NumberType,
				},
			},
			output: "Union<string, number>",
		},
		{
			input: &UnionType{
				ElementTypes: []Type{
					StringType,
				},
				DefaultType: NumberType,
			},
			output: "Union<string, default=number>",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.output, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.output, c.input.String())
		})
	}
}

func TestPackageIdentity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		nameA    string
		versionA string
		nameB    string
		versionB string
		equal    bool
	}{
		{
			nameA: "example",
			nameB: "example",
			equal: true,
		},
		{
			nameA:    "example",
			versionA: "1.0.0",
			nameB:    "example",
			versionB: "1.0.0",
			equal:    true,
		},
		{
			nameA:    "example",
			versionA: "1.2.3-beta",
			nameB:    "example",
			versionB: "1.2.3-beta",
			equal:    true,
		},
		{
			nameA:    "example",
			versionA: "1.2.3-beta+1234",
			nameB:    "example",
			versionB: "1.2.3-beta+1234",
			equal:    true,
		},
		{
			nameA:    "example",
			versionA: "1.0.0",
			nameB:    "example",
		},
		{
			nameA:    "example",
			nameB:    "example",
			versionB: "1.0.0",
		},
		{
			nameA:    "example",
			versionA: "1.0.0",
			nameB:    "example",
			versionB: "1.2.3-beta",
		},
		{
			nameA:    "example",
			versionA: "1.2.3-beta+1234",
			nameB:    "example",
			versionB: "1.2.3-beta+5678",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.nameA, func(t *testing.T) {
			t.Parallel()

			var verA *semver.Version
			if c.versionA != "" {
				v := semver.MustParse(c.versionA)
				verA = &v
			}

			var verB *semver.Version
			if c.versionB != "" {
				v := semver.MustParse(c.versionB)
				verB = &v
			}

			pkgA := &Package{Name: c.nameA, Version: verA}
			pkgB := &Package{Name: c.nameB, Version: verB}
			if c.equal {
				assert.Equal(t, packageIdentity(c.nameA, verA), packageIdentity(c.nameB, verB))
				assert.Equal(t, pkgA.Identity(), pkgB.Identity())
				assert.True(t, pkgA.Equals(pkgB))
			} else {
				assert.NotEqual(t, packageIdentity(c.nameA, verA), packageIdentity(c.nameB, verB))
				assert.NotEqual(t, pkgA.Identity(), pkgB.Identity())
				assert.False(t, pkgA.Equals(pkgB))
			}
		})
	}
}

func TestBindDefaultInt(t *testing.T) {
	t.Parallel()
	dv, diag := bindDefaultValue("fake-path", int(32), nil, IntType)
	if diag.HasErrors() {
		t.Fail()
	}
	assert.Equal(t, int32(32), dv.Value)

	// Check that we error on overflow/underflow when casting int to int32.
	if _, diag := bindDefaultValue("fake-path", int(math.MaxInt64), nil, IntType); !diag.HasErrors() {
		assert.Fail(t, "did not catch oveflow")
		t.Fail()
	}
	if _, diag := bindDefaultValue("fake-path", int(math.MinInt64), nil, IntType); !diag.HasErrors() {
		assert.Fail(t, "did not catch underflow")
	}
}

func TestMarshalResourceWithLanguageSettings(t *testing.T) {
	t.Parallel()

	prop := &Property{
		Name: "prop1",
		Language: map[string]interface{}{
			"csharp": map[string]string{
				"name": "CSharpProp1",
			},
		},
		Type: stringType,
	}
	r := Resource{
		Token: "xyz:index:resource",
		Properties: []*Property{
			prop,
		},
		Language: map[string]interface{}{
			"csharp": map[string]string{
				"name": "CSharpResource",
			},
		},
	}
	p := Package{
		Name:        "xyz",
		DisplayName: "xyz package",
		Version: &semver.Version{
			Major: 0,
			Minor: 0,
			Patch: 0,
		},
		Provider: &Resource{
			IsProvider: true,
			Token:      "provider",
		},
		Resources: []*Resource{
			&r,
		},
	}
	pspec, err := p.MarshalSpec()
	assert.NoError(t, err)
	res, ok := pspec.Resources[r.Token]
	assert.True(t, ok)
	assert.Contains(t, res.Language, "csharp")
	assert.IsType(t, RawMessage{}, res.Language["csharp"])

	prspec, ok := res.Properties[prop.Name]
	assert.True(t, ok)
	assert.Contains(t, prspec.Language, "csharp")
	assert.IsType(t, RawMessage{}, prspec.Language["csharp"])
}

func TestFunctionSpecToJSONAndYAMLTurnaround(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		fspec  FunctionSpec
		serial any
		// For legacy forms, after turning around through serde FunctionSpec will be
		// normalized and not exactly equal to the original; tests will check against the
		// normalized form if provided.
		normalized *FunctionSpec
	}

	ots := &ObjectTypeSpec{
		Type: "object",
		Properties: map[string]PropertySpec{
			"x": {
				TypeSpec: TypeSpec{
					Type: "integer",
				},
			},
		},
	}

	otsPlain := &ObjectTypeSpec{
		Type: "object",
		Properties: map[string]PropertySpec{
			"x": {
				TypeSpec: TypeSpec{
					Type: "integer",
				},
			},
		},
		Plain: []string{"x"},
	}

	testCases := []testCase{
		{
			name: "legacy-outputs-form",
			fspec: FunctionSpec{
				Outputs: ots,
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"properties": map[string]interface{}{
						"x": map[string]interface{}{
							"type": "integer",
						},
					},
					"type": "object",
				},
			},
			normalized: &FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					ObjectTypeSpec: ots,
				},
			},
		},
		{
			name: "legacy-outputs-form-plain-array",
			fspec: FunctionSpec{
				Outputs: otsPlain,
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"properties": map[string]interface{}{
						"x": map[string]interface{}{
							"type": "integer",
						},
					},
					"plain": []interface{}{"x"},
					"type":  "object",
				},
			},
			normalized: &FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					ObjectTypeSpec: otsPlain,
				},
			},
		},
		{
			name: "return-plain-integer",
			fspec: FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					TypeSpec: &TypeSpec{
						Type:  "integer",
						Plain: true,
					},
				},
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"plain": true,
					"type":  "integer",
				},
			},
		},
		{
			name: "return-integer",
			fspec: FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					TypeSpec: &TypeSpec{
						Type: "integer",
					},
				},
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"type": "integer",
				},
			},
		},
		{
			name: "return-plain-object",
			fspec: FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					ObjectTypeSpec:        ots,
					ObjectTypeSpecIsPlain: true,
				},
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"plain": true,
					"properties": map[string]interface{}{
						"x": map[string]interface{}{
							"type": "integer",
						},
					},
					"type": "object",
				},
			},
		},
		{
			name: "return-object",
			fspec: FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					ObjectTypeSpec: ots,
				},
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"properties": map[string]interface{}{
						"x": map[string]interface{}{
							"type": "integer",
						},
					},
					"type": "object",
				},
			},
		},
		{
			name: "return-object-plain-array",
			fspec: FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					ObjectTypeSpec: otsPlain,
				},
			},
			serial: map[string]interface{}{
				"outputs": map[string]interface{}{
					"plain": []interface{}{"x"},
					"properties": map[string]interface{}{
						"x": map[string]interface{}{
							"type": "integer",
						},
					},
					"type": "object",
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		fspec := tc.fspec
		expectSerial := tc.serial
		expectFSpec := fspec
		if tc.normalized != nil {
			expectFSpec = *tc.normalized
		}

		// Test JSON serialization and turnaround.
		t.Run(tc.name+"/json", func(t *testing.T) {
			t.Parallel()
			var serial any

			bytes, err := json.MarshalIndent(fspec, "", "  ")
			require.NoError(t, err)

			err = json.Unmarshal(bytes, &serial)
			require.NoError(t, err)
			require.Equalf(t, expectSerial, serial, "Unexpected JSON serial form")

			var actual FunctionSpec
			err = json.Unmarshal(bytes, &actual)
			require.NoError(t, err)
			require.Equal(t, expectFSpec, actual)
		})

		// Test YAML serialization and turnaround.
		t.Run(tc.name+"/yaml", func(t *testing.T) {
			t.Parallel()
			var serial any

			bytes, err := yaml.Marshal(fspec)
			require.NoError(t, err)

			err = yaml.Unmarshal(bytes, &serial)
			require.NoError(t, err)
			require.Equalf(t, expectSerial, serial, "Unexpected YAML serial form")

			var actual FunctionSpec
			err = yaml.Unmarshal(bytes, &actual)
			require.NoError(t, err)
			require.Equal(t, expectFSpec, actual)
		})
	}
}

func TestFunctionToFunctionSpecTurnaround(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name  string
		fn    *Function
		fspec FunctionSpec
	}

	testCases := []testCase{
		{
			name: "return-type-plain",
			fn: &Function{
				PackageReference: packageDefRef{},
				Token:            "token",
				ReturnType:       IntType,
				ReturnTypePlain:  true,
				Language:         map[string]interface{}{},
			},
			fspec: FunctionSpec{
				ReturnType: &ReturnTypeSpec{
					TypeSpec: &TypeSpec{
						Type:  "integer",
						Plain: true,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name+"/marshalFunction", func(t *testing.T) {
			t.Parallel()
			pkg := Package{}
			fspec, err := pkg.marshalFunction(tc.fn)
			require.NoError(t, err)
			require.Equal(t, tc.fspec, fspec)
		})
		t.Run(tc.name+"/bindFunctionDef", func(t *testing.T) {
			t.Parallel()
			ts := types{
				spec: packageSpecSource{
					&PackageSpec{
						Functions: map[string]FunctionSpec{
							"token": tc.fspec,
						},
					},
				},
				functionDefs: map[string]*Function{},
			}
			fn, diags, err := ts.bindFunctionDef("token")
			require.NoError(t, err)
			require.False(t, diags.HasErrors())
			require.Equal(t, tc.fn, fn)
		})
	}
}
