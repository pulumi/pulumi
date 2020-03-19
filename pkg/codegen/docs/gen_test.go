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
	"testing"

	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/stretchr/testify/assert"
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
	sampleSchema = schema.PackageSpec{
		Name:        "prov",
		Description: "A fake provider's package spec used for testing.",
		Types: map[string]schema.ObjectTypeSpec{
			"prov:/packageLevelFunction:packageLevelFunction": {
				Description: "A package-level function.",
				Type:        "object",
				Properties:  simpleProperties,
			},
			"prov:module/resource:Options": {
				Description: "The options object.",
				Type:        "object",
				Properties:  simpleProperties,
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"prov:module/resource:Resource": {
				InputProperties: map[string]schema.PropertySpec{
					"prop1": {
						Description: "This is prop1's description.",
						TypeSpec: schema.TypeSpec{
							Type: "integer",
						},
					},
					"prop2": {
						Description: "This is prop2's description.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
					"optionsProp": {
						TypeSpec: schema.TypeSpec{
							Type: "object",
							Ref:  "#/types/prov:module/resource:Options",
						},
					},
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"prov:/packageLevelFunction:packageLevelFunction": {
				Description: "A package-level function.",
				Inputs: &schema.ObjectTypeSpec{
					Description: "Inputs",
					Type:        "object",
					Properties:  simpleProperties,
				},
				Outputs: &schema.ObjectTypeSpec{
					Description: "Outputs",
					Properties:  simpleProperties,
					Type:        "object",
				},
			},
		},
	}
)

func TestPythonCasing(t *testing.T) {
	_, err := schema.ImportSpec(sampleSchema)
	assert.NoError(t, err, "importing spec")
}
