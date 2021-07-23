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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
// nolint: lll, goconst
package docs

import (
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

const (
	unitTestTool    = "Pulumi Resource Docs Unit Test"
	providerPackage = "prov"
	codeFence       = "```"
)

var (
	simpleProperties = map[string]schema.PropertySpec{
		"stringProp": {
			Description: "A string prop.",
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
		"boolProp": {
			Description: "A bool prop.",
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}

	// testPackageSpec represents a fake package spec for a Provider used for testing.
	testPackageSpec schema.PackageSpec
)

func initTestPackageSpec(t *testing.T) {
	t.Helper()

	pythonMapCase := map[string]schema.RawMessage{
		"python": schema.RawMessage(`{"mapCase":false}`),
	}
	testPackageSpec = schema.PackageSpec{
		Name:        providerPackage,
		Description: "A fake provider package used for testing.",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Types: map[string]schema.ComplexTypeSpec{
			// Package-level types.
			"prov:/getPackageResourceOptions:getPackageResourceOptions": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "Options object for the package-level function getPackageResource.",
					Type:        "object",
					Properties:  simpleProperties,
				},
			},

			// Module-level types.
			"prov:module/getModuleResourceOptions:getModuleResourceOptions": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "Options object for the module-level function getModuleResource.",
					Type:        "object",
					Properties:  simpleProperties,
				},
			},
			"prov:module/ResourceOptions:ResourceOptions": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "The resource options object.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"stringProp": {
							Description: "A string prop.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"boolProp": {
							Description: "A bool prop.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
						},
						"recursiveType": {
							Description: "I am a recursive type.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
							},
						},
					},
				},
			},
			"prov:module/ResourceOptions2:ResourceOptions2": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "The resource options object.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"uniqueProp": {
							Description: "This is a property unique to this type.",
							Language:    pythonMapCase,
							TypeSpec: schema.TypeSpec{
								Type: "number",
							},
						},
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"prov:module/resource:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: `This is a module-level resource called Resource.
{{% examples %}}
## Example Usage

{{% example %}}
### Basic Example

` + codeFence + `typescript
					// Some TypeScript code.
` + codeFence + `
` + codeFence + `python
					# Some Python code.
` + codeFence + `
{{% /example %}}
{{% example %}}
### Custom Sub-Domain Example

` + codeFence + `typescript
					// Some typescript code
` + codeFence + `
` + codeFence + `python
					# Some Python code.
` + codeFence + `
{{% /example %}}
{{% /examples %}}

## Import 

The import docs would be here

` + codeFence + `sh
$ pulumi import prov:module/resource:Resource test test		
` + codeFence + `
`,
				},
				InputProperties: map[string]schema.PropertySpec{
					"integerProp": {
						Description: "This is integerProp's description.",
						TypeSpec: schema.TypeSpec{
							Type: "integer",
						},
					},
					"stringProp": {
						Description: "This is stringProp's description.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
					"boolProp": {
						Description: "A bool prop.",
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
					"optionsProp": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
						},
					},
					"options2Prop": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions2:ResourceOptions2",
						},
					},
					"recursiveType": {
						Description: "I am a recursive type.",
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/ResourceOptions:ResourceOptions",
						},
					},
				},
			},
			"prov:/packageLevelResource:PackageLevelResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "This is a package-level resource.",
				},
				InputProperties: map[string]schema.PropertySpec{
					"prop": {
						Description: "An input property.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			// Package-level Functions.
			"prov:/getPackageResource:getPackageResource": {
				Description: "A package-level function.",
				Inputs: &schema.ObjectTypeSpec{
					Description: "Inputs for getPackageResource.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"options": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/prov:/getPackageResourceOptions:getPackageResourceOptions",
							},
						},
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Description: "Outputs for getPackageResource.",
					Properties:  simpleProperties,
					Type:        "object",
				},
			},

			// Module-level Functions.
			"prov:module/getModuleResource:getModuleResource": {
				Description: "A module-level function.",
				Inputs: &schema.ObjectTypeSpec{
					Description: "Inputs for getModuleResource.",
					Type:        "object",
					Properties: map[string]schema.PropertySpec{
						"options": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/prov:module/getModuleResource:getModuleResource",
							},
						},
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Description: "Outputs for getModuleResource.",
					Properties:  simpleProperties,
					Type:        "object",
				},
			},
		},
	}
}

// TestResourceNestedPropertyPythonCasing tests that the properties
// of a nested object have the expected casing.
func TestResourceNestedPropertyPythonCasing(t *testing.T) {
	initTestPackageSpec(t)

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	modules := generateModulesFromSchemaPackage(unitTestTool, schemaPkg)
	mod := modules["module"]
	for _, r := range mod.resources {
		nestedTypes := mod.genNestedTypes(r, true)
		if len(nestedTypes) == 0 {
			t.Error("did not find any nested types")
			return
		}

		t.Run("InputPropertiesAreSnakeCased", func(t *testing.T) {
			props := mod.getProperties(r.InputProperties, "python", true, false, false)
			for _, p := range props {
				assert.True(t, strings.Contains(p.Name, "_"), "input property name in python must use snake_case")
			}
		})

		// Non-unique nested properties are ones that have names that occur as direct input properties
		// of the resource or elsewhere in the package and are mapped as snake_case even if the property
		// itself has a "Language" spec with the `MapCase` value of `false`.
		t.Run("NonUniqueNestedProperties", func(t *testing.T) {
			n := nestedTypes[0]
			assert.Equal(t, "Resource<wbr>Options", n.Name, "got %v instead of Resource<wbr>Options", n.Name)

			pyProps := n.Properties["python"]
			nestedObject, ok := testPackageSpec.Types["prov:module/ResourceOptions:ResourceOptions"]
			if !ok {
				t.Error("sample schema package spec does not contain known object type")
				return
			}

			for name := range nestedObject.Properties {
				found := false
				pyName := python.PyName(name)
				for _, prop := range pyProps {
					if prop.Name == pyName {
						found = true
						break
					}
				}

				assert.True(t, found, "expected to find %q", pyName)
			}
		})

		// Unique nested properties are those that only appear inside a nested object and should also be snake_cased.
		t.Run("UniqueNestedProperties", func(t *testing.T) {
			n := nestedTypes[1]
			assert.Equal(t, "Resource<wbr>Options2", n.Name, "got %v instead of Resource<wbr>Options2", n.Name)

			pyProps := n.Properties["python"]
			nestedObject, ok := testPackageSpec.Types["prov:module/ResourceOptions2:ResourceOptions2"]
			if !ok {
				t.Error("sample schema package spec does not contain known object type")
				return
			}

			for name := range nestedObject.Properties {
				found := false
				pyName := python.PyName(name)
				for _, prop := range pyProps {
					if prop.Name == pyName {
						found = true
						break
					}
				}

				assert.True(t, found, "expected to find %q", name)
			}
		})
	}
}

func getResourceFromModule(resource string, mod *modContext) *schema.Resource {
	for _, r := range mod.resources {
		if resourceName(r) != resource {
			continue
		}
		return r
	}
	return nil
}

func getFunctionFromModule(function string, mod *modContext) *schema.Function {
	for _, f := range mod.functions {
		if tokenToName(f.Token) != function {
			continue
		}
		return f
	}
	return nil
}

func TestFunctionHeaders(t *testing.T) {
	initTestPackageSpec(t)

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	tests := []struct {
		ExpectedTitleTag string
		FunctionName     string
		ModuleName       string
		ExpectedMetaDesc string
	}{
		{
			FunctionName: "getPackageResource",
			// Empty string indicates the package-level root module.
			ModuleName:       "",
			ExpectedTitleTag: "prov.getPackageResource",
			ExpectedMetaDesc: "Documentation for the prov.getPackageResource function with examples, input properties, output properties, and supporting types.",
		},
		{
			FunctionName:     "getModuleResource",
			ModuleName:       "module",
			ExpectedTitleTag: "prov.module.getModuleResource",
			ExpectedMetaDesc: "Documentation for the prov.module.getModuleResource function with examples, input properties, output properties, and supporting types.",
		},
	}

	modules := generateModulesFromSchemaPackage(unitTestTool, schemaPkg)
	for _, test := range tests {
		t.Run(test.FunctionName, func(t *testing.T) {
			mod, ok := modules[test.ModuleName]
			if !ok {
				t.Fatalf("could not find the module %s in modules map", test.ModuleName)
			}

			f := getFunctionFromModule(test.FunctionName, mod)
			if f == nil {
				t.Fatalf("could not find %s in modules", test.FunctionName)
			}
			h := mod.genFunctionHeader(f)
			assert.Equal(t, test.ExpectedTitleTag, h.TitleTag)
			assert.Equal(t, test.ExpectedMetaDesc, h.MetaDesc)
		})
	}
}

func TestResourceDocHeader(t *testing.T) {
	initTestPackageSpec(t)

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	tests := []struct {
		Name             string
		ExpectedTitleTag string
		ResourceName     string
		ModuleName       string
		ExpectedMetaDesc string
	}{
		{
			Name:         "PackageLevelResourceHeader",
			ResourceName: "PackageLevelResource",
			// Empty string indicates the package-level root module.
			ModuleName:       "",
			ExpectedTitleTag: "prov.PackageLevelResource",
			ExpectedMetaDesc: "Documentation for the prov.PackageLevelResource resource with examples, input properties, output properties, lookup functions, and supporting types.",
		},
		{
			Name:             "ModuleLevelResourceHeader",
			ResourceName:     "Resource",
			ModuleName:       "module",
			ExpectedTitleTag: "prov.module.Resource",
			ExpectedMetaDesc: "Documentation for the prov.module.Resource resource with examples, input properties, output properties, lookup functions, and supporting types.",
		},
	}

	modules := generateModulesFromSchemaPackage(unitTestTool, schemaPkg)
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			mod, ok := modules[test.ModuleName]
			if !ok {
				t.Fatalf("could not find the module %s in modules map", test.ModuleName)
			}

			r := getResourceFromModule(test.ResourceName, mod)
			if r == nil {
				t.Fatalf("could not find %s in modules", test.ResourceName)
			}
			h := mod.genResourceHeader(r)
			assert.Equal(t, test.ExpectedTitleTag, h.TitleTag)
			assert.Equal(t, test.ExpectedMetaDesc, h.MetaDesc)
		})
	}
}

func TestExamplesProcessing(t *testing.T) {
	initTestPackageSpec(t)

	description := testPackageSpec.Resources["prov:module/resource:Resource"].Description
	docInfo := decomposeDocstring(description)
	examplesSection := docInfo.examples
	importSection := docInfo.importDetails

	assert.NotEmpty(t, importSection)

	// The resource under test has two examples and both have TS and Python examples.
	assert.Equal(t, 2, len(examplesSection))
	assert.Equal(t, "### Basic Example", examplesSection[0].Title)
	assert.Equal(t, "### Custom Sub-Domain Example", examplesSection[1].Title)
	expectedLangSnippets := []string{"typescript", "python"}
	otherLangSnippets := []string{"csharp", "go"}
	for _, e := range examplesSection {
		for _, lang := range expectedLangSnippets {
			_, ok := e.Snippets[lang]
			assert.True(t, ok, "Could not find %s snippet", lang)
		}
		for _, lang := range otherLangSnippets {
			snippet, ok := e.Snippets[lang]
			assert.True(t, ok, "Expected to find default placeholders for other languages")
			assert.Contains(t, "Coming soon!", snippet)
		}
	}
}

func generatePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	return GeneratePackage(tool, pkg)
}

func TestGeneratePackage(t *testing.T) {
	test.TestSDKCodegen(t, "docs", generatePackage)
}
