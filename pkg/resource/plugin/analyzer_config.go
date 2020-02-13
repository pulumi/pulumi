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
	"io/ioutil"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/xeipuuv/gojsonschema"
)

// LoadPolicyPackConfigFromFile loads the JSON config from a file.
func LoadPolicyPackConfigFromFile(file string) (map[string]AnalyzerPolicyConfig, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parsePolicyPackConfig(b)
}

func parsePolicyPackConfig(b []byte) (map[string]AnalyzerPolicyConfig, error) {
	result := make(map[string]AnalyzerPolicyConfig)

	// Gracefully allow empty content.
	if strings.TrimSpace(string(b)) == "" {
		return nil, nil
	}

	config := make(map[string]interface{})
	if err := json.Unmarshal(b, &config); err != nil {
		return nil, err
	}
	for k, v := range config {
		var enforcementLevel apitype.EnforcementLevel
		var properties map[string]interface{}
		switch val := v.(type) {
		case string:
			el := apitype.EnforcementLevel(val)
			if !el.IsValid() {
				return nil, errors.Errorf("Value %q for %q is not a valid enforcement level", val, k)
			}
			enforcementLevel = el
		case map[string]interface{}:
			if elUnknown, hasEnforcementLevel := val["enforcementLevel"]; hasEnforcementLevel {
				elStr, isStr := elUnknown.(string)
				if !isStr {
					return nil, errors.Errorf("Value %v for %q is not a valid enforcement level", elUnknown, k)
				}
				el := apitype.EnforcementLevel(elStr)
				if !el.IsValid() {
					return nil, errors.Errorf("Value %q for %q is not a valid enforcement level", elStr, k)
				}
				enforcementLevel = el
				// Remove enforcementLevel from the map.
				delete(val, "enforcementLevel")
			}
			if len(val) > 0 {
				properties = val
			}
		default:
			return nil, errors.Errorf("Value %v for %q is not a valid value; must be a string or object", v, k)
		}

		// Don't bother including empty configs.
		if enforcementLevel == "" && len(properties) == 0 {
			continue
		}

		result[k] = AnalyzerPolicyConfig{
			EnforcementLevel: enforcementLevel,
			Properties:       properties,
		}
	}
	return result, nil
}

// ValidatePolicyPackConfig validates the policy pack's configuration.
func ValidatePolicyPackConfig(policies []AnalyzerPolicyInfo, config map[string]AnalyzerPolicyConfig) error {
	contract.Assertf(config != nil, "contract != nil")
	var allErrors *multierror.Error
	for _, policy := range policies {
		if policy.ConfigSchema == nil {
			continue
		}
		var props map[string]interface{}
		if c, ok := config[policy.Name]; ok {
			props = c.Properties
		}
		if props == nil {
			props = make(map[string]interface{})
		}
		validationErrors, err := validatePolicyConfig(*policy.ConfigSchema, props)
		if err != nil {
			allErrors = multierror.Append(allErrors, err)
		}
		for _, validationError := range validationErrors {
			allErrors = multierror.Append(allErrors, errors.Errorf("Policy %q: %s", policy.Name, validationError))
		}
	}
	return allErrors.ErrorOrNil()
}

// validatePolicyConfig validates an individual policy's configuration.
func validatePolicyConfig(schema AnalyzerPolicyConfigSchema, config map[string]interface{}) ([]string, error) {
	var errors []string
	schemaLoader := gojsonschema.NewGoLoader(convertSchema(schema))
	documentLoader := gojsonschema.NewGoLoader(config)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, err
	}
	if !result.Valid() {
		for _, err := range result.Errors() {
			// Root errors are prefixed with "(root):" (e.g. "(root): foo is required"),
			// but that's just noise for our purposes, so we trim it from the message.
			msg := strings.TrimPrefix(err.String(), "(root): ")
			errors = append(errors, msg)
		}
	}
	return errors, nil
}

func convertSchema(schema AnalyzerPolicyConfigSchema) JSONSchema {
	result := JSONSchema{}
	result["type"] = "object"
	if len(schema.Properties) > 0 {
		result["properties"] = schema.Properties
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	return result
}

// CreateConfigWithDefaults returns a new map filled-in with defaults from the policy metadata.
func CreateConfigWithDefaults(policies []AnalyzerPolicyInfo) (map[string]AnalyzerPolicyConfig, error) {
	result := make(map[string]AnalyzerPolicyConfig)

	// Prepare the resulting config with all defaults from the policy metadata.
	for _, policy := range policies {
		var props map[string]interface{}

		// Set default values from the schema.
		if policy.ConfigSchema != nil {
			props = make(map[string]interface{})
			for k, v := range policy.ConfigSchema.Properties {
				if val, ok := v["default"]; ok {
					props[k] = val
				}
			}
		}

		result[policy.Name] = AnalyzerPolicyConfig{
			EnforcementLevel: policy.EnforcementLevel,
			Properties:       props,
		}
	}

	return result, nil
}

// ReconcilePolicyPackConfig takes metadata about each policy containing default values and config schema, and
// reconciles this with the given config to produce a new config that has all default values filled-in and then sets
// configured values.
func ReconcilePolicyPackConfig(
	policies []AnalyzerPolicyInfo, config map[string]AnalyzerPolicyConfig) (map[string]AnalyzerPolicyConfig, error) {
	// Prepare the resulting config with all defaults from the policy metadata.
	result, err := CreateConfigWithDefaults(policies)
	contract.Assertf(result != nil, "result != nil")
	if err != nil {
		return nil, err
	}

	// Next, if the given config has "all" and an enforcement level, set it for all
	// policies.
	if config != nil {
		if all, hasAll := config["all"]; hasAll && all.EnforcementLevel.IsValid() {
			for _, v := range result {
				v.EnforcementLevel = all.EnforcementLevel
			}
		}
	}

	// Next, loop through the given config, and set values.
	for policy, givenConfig := range config {
		resultConfig, hasResultConfig := result[policy]
		if !hasResultConfig {
			continue
		}

		if givenConfig.EnforcementLevel != "" {
			resultConfig.EnforcementLevel = givenConfig.EnforcementLevel
		}
		if len(givenConfig.Properties) > 0 && resultConfig.Properties == nil {
			resultConfig.Properties = make(map[string]interface{})
		}
		for k, v := range givenConfig.Properties {
			resultConfig.Properties[k] = v
		}
	}

	// Validate the resulting config.
	if config != nil {
		if err := ValidatePolicyPackConfig(policies, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}
