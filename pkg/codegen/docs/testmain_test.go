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
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/codegen/schema"
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

func initTestPackageSpec() {
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
			"prov:module/SomeResourceOptions:SomeResourceOptions": {
				Description: "The resource options object.",
				Type:        "object",
				Properties:  simpleProperties,
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
					"optionsProp": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/prov:module/SomeResourceOptions:SomeResourceOptions",
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

func TestMain(m *testing.M) {
	initTestPackageSpec()
	os.Exit(m.Run())
}
