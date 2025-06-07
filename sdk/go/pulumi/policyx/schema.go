// Copyright 2025, Pulumi Corporation.
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

package policyx

type EnforcementLevel int32

const (
	EnforcementLevelAdvisory  EnforcementLevel = 0 // Displayed to users, but does not block deployment.
	EnforcementLevelMandatory EnforcementLevel = 1 // Stops deployment, cannot be overridden.
	EnforcementLevelDisabled  EnforcementLevel = 2 // Disabled policies do not run during a deployment.
)

type PolicyConfigJSONSchemaTypes []PolicyConfigJSONSchemaType

type PolicyConfigJSONSchemaType string

const (
	PolicyConfigJSONSchemaTypeBoolean PolicyConfigJSONSchemaType = "boolean"
	PolicyConfigJSONSchemaTypeNumber  PolicyConfigJSONSchemaType = "number"
	PolicyConfigJSONSchemaTypeNull    PolicyConfigJSONSchemaType = "null"
	PolicyConfigJSONSchemaTypeObject  PolicyConfigJSONSchemaType = "object"
	PolicyConfigJSONSchemaTypeString  PolicyConfigJSONSchemaType = "string"
)

type PolicyConfigJSONSchemaTypeName string

const (
	PolicyConfigJSONSchemaTypeNameString  PolicyConfigJSONSchemaTypeName = "string"
	PolicyConfigJSONSchemaTypeNameNumber  PolicyConfigJSONSchemaTypeName = "number"
	PolicyConfigJSONSchemaTypeNameInteger PolicyConfigJSONSchemaTypeName = "integer"
	PolicyConfigJSONSchemaTypeNameBoolean PolicyConfigJSONSchemaTypeName = "boolean"
	PolicyConfigJSONSchemaTypeNameObject  PolicyConfigJSONSchemaTypeName = "object"
	PolicyConfigJSONSchemaTypeNameArray   PolicyConfigJSONSchemaTypeName = "array"
	PolicyConfigJSONSchemaTypeNameNull    PolicyConfigJSONSchemaTypeName = "null"
)

// PolicyConfigJSONSchema represents a JSON schema for policy configuration.
type PolicyConfigJSONSchema struct {
	Types []PolicyConfigJSONSchemaTypeName `json:"types"`
	Enum  []PolicyConfigJSONSchemaType     `json:"enum"`
	Const []PolicyConfigJSONSchemaType     `json:"const"`

	MultipleOf       *int `json:"multipleOf,omitempty"`
	Maximum          *int `json:"maximum,omitempty"`
	ExclusiveMaximum *int `json:"exclusiveMaximum,omitempty"`
	Minimum          *int `json:"minimum,omitempty"`
	ExclusiveMinimum *int `json:"exclusiveMinimum,omitempty"`

	MaxLength *int    `json:"maxLength,omitempty"`
	MinLength *int    `json:"minLength,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`

	Items                []*PolicyConfigJSONSchema          `json:"items,omitempty"`
	AdditionalItems      *PolicyConfigJSONSchema            `json:"additionalItems,omitempty"`
	MaxItems             *int                               `json:"maxItems,omitempty"`
	MinItems             *int                               `json:"minItems,omitempty"`
	UniqueItems          *bool                              `json:"uniqueItems,omitempty"`
	Contains             *PolicyConfigJSONSchema            `json:"contains,omitempty"`
	MaxProperties        *int                               `json:"maxProperties,omitempty"`
	MinProperties        *int                               `json:"minProperties,omitempty"`
	Required             []string                           `json:"required,omitempty"`
	Properties           map[string]*PolicyConfigJSONSchema `json:"properties,omitempty"`
	PatternProperties    map[string]*PolicyConfigJSONSchema `json:"patternProperties,omitempty"`
	AdditionalProperties *PolicyConfigJSONSchema            `json:"additionalProperties,omitempty"`
	Dependencies         map[string]*PolicyConfigJSONSchema `json:"dependencies,omitempty"`
	PropertyNames        *PolicyConfigJSONSchema            `json:"propertyNames,omitempty"`
	Format               *string                            `json:"format,omitempty"`

	Description *string                     `json:"description,omitempty"`
	Default     *PolicyConfigJSONSchemaType `json:"default,omitempty"`
}

// PolicyConfigSchema represents the configuration schema for a policy.
type PolicyConfigSchema struct {
	// The policy's configuration properties.
	Properties map[string]PolicyConfigJSONSchema `json:"properties"`

	// The configuration properties that are required.
	Required []string `json:"required"`
}
