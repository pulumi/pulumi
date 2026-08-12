// Copyright 2026, Pulumi Corporation.
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

package schema

// JSONSchemaSchema is a schema that validates JSON schema definitions.
func JSONSchemaSchema() *Schema {
	return &Schema{
		Defs: map[string]*Schema{
			"schema": {
				AnyOf: []*Schema{
					// Boolean schema (true = accept all, false = reject all)
					{Type: "boolean"},
					// Object schema
					{
						Type: "object",
						Properties: map[string]*Schema{
							// Core vocabulary
							"$defs": {
								Type:                 "object",
								AdditionalProperties: &Schema{Ref: "#/$defs/schema"},
							},

							// Applicator vocabulary
							"$ref":  {Type: "string"},
							"anyOf": {Type: "array", Items: &Schema{Ref: "#/$defs/schema"}},
							"oneOf": {Type: "array", Items: &Schema{Ref: "#/$defs/schema"}},
							"prefixItems": {
								Type:  "array",
								Items: &Schema{Ref: "#/$defs/schema"},
							},
							"items":                {Ref: "#/$defs/schema"},
							"additionalProperties": {Ref: "#/$defs/schema"},
							"properties": {
								Type:                 "object",
								AdditionalProperties: &Schema{Ref: "#/$defs/schema"},
							},

							// Validation vocabulary
							"type": {
								Type: "string",
								Enum: []any{"string", "number", "boolean", "array", "object", "null"},
							},
							"const":            {}, // Any value
							"enum":             {Type: "array"},
							"multipleOf":       {Type: "number"},
							"maximum":          {Type: "number"},
							"exclusiveMaximum": {Type: "number"},
							"minimum":          {Type: "number"},
							"exclusiveMinimum": {Type: "number"},
							"maxLength":        {Type: "number"},
							"minLength":        {Type: "number"},
							"pattern":          {Type: "string"},
							"maxItems":         {Type: "number"},
							"minItems":         {Type: "number"},
							"uniqueItems":      {Type: "boolean"},
							"maxProperties":    {Type: "number"},
							"minProperties":    {Type: "number"},
							"required": {
								Type:  "array",
								Items: &Schema{Type: "string"},
							},
							"dependentRequired": {
								Type:                 "object",
								AdditionalProperties: &Schema{Type: "array", Items: &Schema{Type: "string"}},
							},

							// Metadata vocabulary
							"title":       {Type: "string"},
							"description": {Type: "string"},
							"default":     {}, // Any value
							"deprecated":  {Type: "boolean"},
							"examples":    {Type: "array"},
						},
					},
				},
			},
		},
		Ref: "#/$defs/schema",
	}
}
