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

func TestCreateConfigWithDefaultsEnforcementLevel(t *testing.T) {
	tests := []struct {
		Policies []AnalyzerPolicyInfo
		Expected map[string]AnalyzerPolicyConfig
	}{
		{
			Policies: []AnalyzerPolicyInfo{
				{
					Name:             "policy",
					EnforcementLevel: "advisory",
				},
			},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			result, err := CreateConfigWithDefaults(test.Policies)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestCreateConfigWithDefaults(t *testing.T) {
	tests := []struct {
		Policies []AnalyzerPolicyInfo
		Expected map[string]AnalyzerPolicyConfig
	}{
		{
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
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			result, err := CreateConfigWithDefaults(test.Policies)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestValidatePolicyConfig(t *testing.T) {
	tests := []struct {
		Test     string
		Schema   AnalyzerPolicyConfigSchema
		Config   map[string]interface{}
		Expected []string
	}{
		{
			Test: "Required property missing",
			Schema: AnalyzerPolicyConfigSchema{
				Properties: map[string]JSONSchema{
					"foo": {
						"type": "string",
					},
				},
				Required: []string{"foo"},
			},
			Config:   map[string]interface{}{},
			Expected: []string{"foo is required"},
		},
		{
			Test: "Invalid type",
			Schema: AnalyzerPolicyConfigSchema{
				Properties: map[string]JSONSchema{
					"foo": {
						"type": "string",
					},
				},
			},
			Config: map[string]interface{}{
				"foo": 1,
			},
			Expected: []string{"foo: Invalid type. Expected: string, given: integer"},
		},
		{
			Test: "Invalid enum value",
			Schema: AnalyzerPolicyConfigSchema{
				Properties: map[string]JSONSchema{
					"foo": {
						"type": "string",
						"enum": []string{"bar", "baz"},
					},
				},
			},
			Config: map[string]interface{}{
				"foo": "blah",
			},
			Expected: []string{"foo: foo must be one of the following: \"bar\", \"baz\""},
		},
	}

	for _, test := range tests {
		t.Run(test.Test, func(t *testing.T) {
			result, err := validatePolicyConfig(test.Schema, test.Config)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestReconcileSuccess(t *testing.T) {
	tests := []struct {
		Test     string
		Policies []AnalyzerPolicyInfo
		Config   map[string]AnalyzerPolicyConfig
		Expected map[string]AnalyzerPolicyConfig
	}{
		{
			Test: "Default value specified in schema used.",
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
			Config: map[string]AnalyzerPolicyConfig{},
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
			Test: "Default value specified in schema for required field used.",
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
			Config: map[string]AnalyzerPolicyConfig{},
			Expected: map[string]AnalyzerPolicyConfig{
				"policy": {
					EnforcementLevel: "advisory",
					Properties: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Test, func(t *testing.T) {
			result, err := ReconcilePolicyPackConfig(test.Policies, test.Config)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, result)
		})
	}
}

func TestReconcileFail(t *testing.T) {
	tests := []struct {
		Test          string
		Policies      []AnalyzerPolicyInfo
		Config        map[string]AnalyzerPolicyConfig
		ExpectedError string
	}{
		{
			Test: "Required config property not set.",
			Policies: []AnalyzerPolicyInfo{
				{
					Name: "foo-policy",
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
			Config:        map[string]AnalyzerPolicyConfig{},
			ExpectedError: "1 error occurred:\n\t* Policy \"foo-policy\": foo is required\n\n",
		},
		{
			Test: "Required config property with default value set to incorrect type.",
			Policies: []AnalyzerPolicyInfo{
				{
					Name: "foo-policy",
					ConfigSchema: &AnalyzerPolicyConfigSchema{
						Properties: map[string]JSONSchema{
							"foo": {
								"type":    "string",
								"default": 1,
							},
						},
						Required: []string{"foo"},
					},
				},
			},
			Config: map[string]AnalyzerPolicyConfig{},
			ExpectedError: "1 error occurred:\n\t* Policy \"foo-policy\": " +
				"foo: Invalid type. Expected: string, given: integer\n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.Test, func(t *testing.T) {
			result, err := ReconcilePolicyPackConfig(test.Policies, test.Config)
			assert.Nil(t, result)
			assert.Error(t, err)
			if test.ExpectedError != "" {
				assert.Equal(t, test.ExpectedError, err.Error())
			}
		})
	}
}
