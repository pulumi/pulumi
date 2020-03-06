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
package plugin

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/stretchr/testify/assert"
)

type JSONTestCaseSuccess struct {
	JSON     string
	Expected map[string]AnalyzerPolicyConfig
}

var success = []JSONTestCaseSuccess{
	{
		JSON:     `{}`,
		Expected: map[string]AnalyzerPolicyConfig{},
	},
	{
		JSON: `{"foo":{"enforcementLevel":"advisory"}}`,
		Expected: map[string]AnalyzerPolicyConfig{
			"foo": {
				EnforcementLevel: apitype.Advisory,
			},
		},
	},
	{
		JSON: `{"foo":{"enforcementLevel":"mandatory"}}`,
		Expected: map[string]AnalyzerPolicyConfig{
			"foo": {
				EnforcementLevel: apitype.Mandatory,
			},
		},
	},
	{
		JSON: `{"foo":{"enforcementLevel":"advisory","bar":"blah"}}`,
		Expected: map[string]AnalyzerPolicyConfig{
			"foo": {
				EnforcementLevel: apitype.Advisory,
				Properties: map[string]interface{}{
					"bar": "blah",
				},
			},
		},
	},
	{
		JSON:     `{"foo":{}}`,
		Expected: map[string]AnalyzerPolicyConfig{},
	},
	{
		JSON: `{"foo":{"bar":"blah"}}`,
		Expected: map[string]AnalyzerPolicyConfig{
			"foo": {
				Properties: map[string]interface{}{
					"bar": "blah",
				},
			},
		},
	},
	{
		JSON: `{"policy1":{"foo":"one"},"policy2":{"foo":"two"}}`,
		Expected: map[string]AnalyzerPolicyConfig{
			"policy1": {
				Properties: map[string]interface{}{
					"foo": "one",
				},
			},
			"policy2": {
				Properties: map[string]interface{}{
					"foo": "two",
				},
			},
		},
	},
}

func TestParsePolicyPackConfigFromAPISuccess(t *testing.T) {
	for _, test := range success {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			config := make(map[string]*json.RawMessage)
			unmarshalErr := json.Unmarshal([]byte(test.JSON), &config)
			assert.NoError(t, unmarshalErr)

			result, err := ParsePolicyPackConfigFromAPI(config)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestParsePolicyPackConfigSuccess(t *testing.T) {
	tests := []JSONTestCaseSuccess{
		{
			JSON:     "",
			Expected: nil,
		},
		{
			JSON:     "    ",
			Expected: nil,
		},
		{
			JSON:     "\t",
			Expected: nil,
		},
		{
			JSON:     "\n",
			Expected: nil,
		},
		{
			JSON: `{"foo":"advisory"}`,
			Expected: map[string]AnalyzerPolicyConfig{
				"foo": {
					EnforcementLevel: apitype.Advisory,
				},
			},
		},
		{
			JSON: `{"foo":"mandatory"}`,
			Expected: map[string]AnalyzerPolicyConfig{
				"foo": {
					EnforcementLevel: apitype.Mandatory,
				},
			},
		},
		{
			JSON: `{"all":"mandatory","policy1":{"foo":"one"},"policy2":{"foo":"two"}}`,
			Expected: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: apitype.Mandatory,
				},
				"policy1": {
					Properties: map[string]interface{}{
						"foo": "one",
					},
				},
				"policy2": {
					Properties: map[string]interface{}{
						"foo": "two",
					},
				},
			},
		},
	}
	tests = append(tests, success...)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			result, err := parsePolicyPackConfig([]byte(test.JSON))
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestParsePolicyPackConfigFail(t *testing.T) {
	tests := []string{
		`{"foo":[]}`,
		`{"foo":null}`,
		`{"foo":undefined}`,
		`{"foo":0}`,
		`{"foo":true}`,
		`{"foo":false}`,
		`{"foo":""}`,
		`{"foo":"bar"}`,
		`{"foo":{"enforcementLevel":[]}}`,
		`{"foo":{"enforcementLevel":null}}`,
		`{"foo":{"enforcementLevel":undefined}}`,
		`{"foo":{"enforcementLevel":0}}`,
		`{"foo":{"enforcementLevel":true}}`,
		`{"foo":{"enforcementLevel":false}}`,
		`{"foo":{"enforcementLevel":{}}}`,
		`{"foo":{"enforcementLevel":""}}`,
		`{"foo":{"enforcementLevel":"bar"}}`,
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			result, err := parsePolicyPackConfig([]byte(test))
			assert.Nil(t, result)
			assert.Error(t, err)
		})
	}
}

func TestExtractEnforcementLevelSuccess(t *testing.T) {
	tests := []struct {
		Properties               map[string]interface{}
		ExpectedEnforcementLevel apitype.EnforcementLevel
		ExpectedProperties       map[string]interface{}
	}{
		{
			Properties:               map[string]interface{}{},
			ExpectedEnforcementLevel: "",
			ExpectedProperties:       map[string]interface{}{},
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": "advisory",
			},
			ExpectedEnforcementLevel: "advisory",
			ExpectedProperties:       map[string]interface{}{},
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": "mandatory",
			},
			ExpectedEnforcementLevel: "mandatory",
			ExpectedProperties:       map[string]interface{}{},
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": "disabled",
			},
			ExpectedEnforcementLevel: "disabled",
			ExpectedProperties:       map[string]interface{}{},
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": "advisory",
				"foo":              "bar",
				"blah":             1,
			},
			ExpectedEnforcementLevel: "advisory",
			ExpectedProperties: map[string]interface{}{
				"foo":  "bar",
				"blah": 1,
			},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			result, err := extractEnforcementLevel(test.Properties)
			assert.NoError(t, err)
			assert.Equal(t, test.ExpectedEnforcementLevel, result)
			assert.Equal(t, test.ExpectedProperties, test.Properties)
		})
	}
}

func TestExtractEnforcementLevelFail(t *testing.T) {
	tests := []struct {
		Properties    map[string]interface{}
		ExpectedError string
	}{
		{
			Properties: map[string]interface{}{
				"enforcementLevel": "",
			},
			ExpectedError: `"" is not a valid enforcement level`,
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": "foo",
			},
			ExpectedError: `"foo" is not a valid enforcement level`,
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": nil,
			},
			ExpectedError: `<nil> is not a valid enforcement level; must be a string`,
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": 1,
			},
			ExpectedError: `1 is not a valid enforcement level; must be a string`,
		},
		{
			Properties: map[string]interface{}{
				"enforcementLevel": []string{},
			},
			ExpectedError: `[] is not a valid enforcement level; must be a string`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			result, err := extractEnforcementLevel(test.Properties)
			assert.Equal(t, apitype.EnforcementLevel(""), result)
			assert.Error(t, err)
			if test.ExpectedError != "" {
				assert.Equal(t, test.ExpectedError, err.Error())
			}
		})
	}
}

func TestReconcilePolicyPackConfigSuccess(t *testing.T) {
	tests := []struct {
		Test     string
		Policies []AnalyzerPolicyInfo
		Config   map[string]AnalyzerPolicyConfig
		Expected map[string]AnalyzerPolicyConfig
	}{
		{
			Test: "Default enforcement level used",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "mandatory",
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "mandatory",
				},
			},
		},
		{
			Test: "Specified enforcement level used",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "mandatory",
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
				},
			},
		},
		{
			Test: "Enforcement level from 'all' used",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "disabled",
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: "mandatory",
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: "mandatory",
				},
				"policy": {
					EnforcementLevel: "mandatory",
				},
			},
		},
		{
			Test: "Enforcement level from 'all' used with multiple policies",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy-one",
					EnforcementLevel: "advisory",
				},
				{
					Name:             "policy-two",
					EnforcementLevel: "mandatory",
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: "disabled",
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: "disabled",
				},
				"policy-one": {
					EnforcementLevel: "disabled",
				},
				"policy-two": {
					EnforcementLevel: "disabled",
				},
			},
		},
		{
			Test: "Specified config enforcement level used even if 'all' is present",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "disabled",
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: "mandatory",
				},
				"policy": {
					EnforcementLevel: "advisory",
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"all": {
					EnforcementLevel: "mandatory",
				},
				"policy": {
					EnforcementLevel: "advisory",
				},
			},
		},
		{
			Test: "Default string value specified in schema used",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "string",
								"default": "bar",
							},
						},
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		{
			Test: "Default number value specified in schema used",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "number",
								"default": float64(42),
							},
						},
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(42),
					},
				},
			},
		},
		{
			Test: "Specified config value overrides default value",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "string",
								"default": "bar",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "overridden",
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "overridden",
					},
				},
			},
		},
		{
			Test: "Default value specified in schema for required field used",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "string",
								"default": "bar",
							},
						},
						Required: []string{"foo"},
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		{
			Test: "type: string",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "string",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		{
			Test: "type: number (int)",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "number",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(42),
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(42),
					},
				},
			},
		},
		{
			Test: "type: number (float)",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "number",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(3.14),
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(3.14),
					},
				},
			},
		},
		{
			Test: "type: integer",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "integer",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(42),
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": float64(42),
					},
				},
			},
		},
		{
			Test: "type: boolean true",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "boolean",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": true,
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": true,
					},
				},
			},
		},
		{
			Test: "type: boolean false",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "boolean",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": false,
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": false,
					},
				},
			},
		},
		{
			Test: "type: object",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "object",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": map[string]interface{}{"bar": "baz"},
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": map[string]interface{}{"bar": "baz"},
					},
				},
			},
		},
		{
			Test: "type: array",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "array",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": []string{"a", "b", "c"},
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": []string{"a", "b", "c"},
					},
				},
			},
		},
		{
			Test: "type: null",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "null",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": nil,
					},
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": nil,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Test, func(t *testing.T) {
			result, validationErrors, err := ReconcilePolicyPackConfig(test.Policies, test.Config)
			assert.NoError(t, err)
			assert.Empty(t, validationErrors)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestReconcilePolicyPackConfigValidationErrors(t *testing.T) {
	tests := []struct {
		Test                     string
		Policies                 []AnalyzerPolicyInfo
		Config                   map[string]AnalyzerPolicyConfig
		ExpectedValidationErrors []string
	}{
		{
			Test: "Required config property not set",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "string",
							},
						},
						Required: []string{"foo"},
					},
				},
			},
			ExpectedValidationErrors: []string{"foo-policy: foo is required"},
		},
		{
			Test: "Default value set to incorrect type",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "string",
								"default": 1,
							},
						},
					},
				},
			},
			ExpectedValidationErrors: []string{"foo-policy: foo: Invalid type. Expected: string, given: integer"},
		},
		{
			Test: "Default value too long",
			Policies: []AnalyzerPolicyInfo{
				{
					Name: "foo-policy",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":      "string",
								"maxLength": 3,
								"default":   "this value is too long",
							},
						},
					},
				},
			},
			ExpectedValidationErrors: []string{"foo-policy: foo: String length must be less than or equal to 3"},
		},
		{
			Test: "Default value too short",
			Policies: []AnalyzerPolicyInfo{
				{
					Name: "foo-policy",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":      "string",
								"minLength": 50,
								"default":   "this value is too short",
							},
						},
					},
				},
			},
			ExpectedValidationErrors: []string{"foo-policy: foo: String length must be greater than or equal to 50"},
		},
		{
			Test: "Default value set to invalid enum value",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "string",
								"enum":    []string{"bar", "baz"},
								"default": "blah",
							},
						},
					},
				},
			},
			ExpectedValidationErrors: []string{`foo-policy: foo: foo must be one of the following: "bar", "baz"`},
		},
		{
			Test: "Default value set to invalid constant value",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"const":   "bar",
								"default": "blah",
							},
						},
					},
				},
			},
			ExpectedValidationErrors: []string{`foo-policy: foo: foo does not match: "bar"`},
		},
		{
			Test: "Incorrect type",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "string",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"foo-policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": 1,
					},
				},
			},
			ExpectedValidationErrors: []string{"foo-policy: foo: Invalid type. Expected: string, given: integer"},
		},
		{
			Test: "Invalid enum value",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type": "string",
								"enum": []string{"bar", "baz"},
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"foo-policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "blah",
					},
				},
			},
			ExpectedValidationErrors: []string{`foo-policy: foo: foo must be one of the following: "bar", "baz"`},
		},
		{
			Test: "Invalid constant value",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"const": "bar",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"foo-policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "blah",
					},
				},
			},
			ExpectedValidationErrors: []string{`foo-policy: foo: foo does not match: "bar"`},
		},
		{
			Test: "Multiple validation errors",
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "foo-policy",
					EnforcementLevel: "advisory",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":      "string",
								"maxLength": 3,
							},
							"bar": {
								"type": "integer",
							},
						},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{
				"foo-policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "this is too long",
						"bar": float64(3.14),
					},
				},
			},
			ExpectedValidationErrors: []string{
				"foo-policy: bar: Invalid type. Expected: integer, given: number",
				"foo-policy: foo: String length must be less than or equal to 3",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Test, func(t *testing.T) {
			result, validationErrors, err := ReconcilePolicyPackConfig(test.Policies, test.Config)
			assert.NoError(t, err)
			assert.Nil(t, result)
			assert.ElementsMatch(t, test.ExpectedValidationErrors, validationErrors)
		})
	}
}
