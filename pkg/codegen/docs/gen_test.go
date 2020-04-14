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
	"encoding/json"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v2/codegen/python"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/stretchr/testify/assert"
)

const (
	unitTestTool    = "Pulumi Resource Docs Unit Test"
	providerPackage = "prov"
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

	pythonMapCase := map[string]json.RawMessage{
		"python": json.RawMessage(`{"mapCase":false}`),
	}
	testPackageSpec = schema.PackageSpec{
		Name:        providerPackage,
		Description: "A fake provider package used for testing.",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Types: map[string]schema.ObjectTypeSpec{
			// Package-level types.
			"prov:/getPackageResourceOptions:getPackageResourceOptions": {
				Description: "Options object for the package-level function getPackageResource.",
				Type:        "object",
				Properties:  simpleProperties,
			},

			// Module-level types.
			"prov:module/getModuleResourceOptions:getModuleResourceOptions": {
				Description: "Options object for the module-level function getModuleResource.",
				Type:        "object",
				Properties:  simpleProperties,
			},
			"prov:module/ResourceOptions:ResourceOptions": {
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
				},
			},
			"prov:module/ResourceOptions2:ResourceOptions2": {
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
		Resources: map[string]schema.ResourceSpec{
			"prov:module/resource:Resource": {
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

	schemaPkg, err := schema.ImportSpec(testPackageSpec)
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
			props := mod.getProperties(r.InputProperties, "python", true, false)
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

		// Unique nested properties are those that only appear inside a nested object and therefore
		// are never mapped to their snake_case. Therefore, such properties must be rendered with a
		// camelCase.
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
				for _, prop := range pyProps {
					if prop.Name == name {
						found = true
						break
					}
				}

				assert.True(t, found, "expected to find %q", name)
			}
		})
	}
}
