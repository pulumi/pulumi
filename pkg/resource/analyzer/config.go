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

package analyzer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/xeipuuv/gojsonschema"
)

// LoadPolicyPackConfigFromFile loads the JSON config from a file.
func LoadPolicyPackConfigFromFile(file string) (map[string]plugin.AnalyzerPolicyConfig, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parsePolicyPackConfig(b)
}

// ParsePolicyPackConfigFromAPI parses the config returned from the service.
func ParsePolicyPackConfigFromAPI(config map[string]*json.RawMessage) (map[string]plugin.AnalyzerPolicyConfig, error) {
	result := map[string]plugin.AnalyzerPolicyConfig{}
	for k, v := range config {
		if v == nil {
			continue
		}

		var enforcementLevel apitype.EnforcementLevel
		var properties map[string]interface{}

		props := make(map[string]interface{})
		if err := json.Unmarshal(*v, &props); err != nil {
			return nil, err
		}

		el, err := extractEnforcementLevel(props)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing enforcement level for %q", k)
		}
		enforcementLevel = el
		if len(props) > 0 {
			properties = props
		}

		// Don't bother including empty configs.
		if enforcementLevel == "" && len(properties) == 0 {
			continue
		}

		result[k] = plugin.AnalyzerPolicyConfig{
			EnforcementLevel: enforcementLevel,
			Properties:       properties,
		}
	}
	return result, nil
}

func parsePolicyPackConfig(b []byte) (map[string]plugin.AnalyzerPolicyConfig, error) {
	result := make(map[string]plugin.AnalyzerPolicyConfig)

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
				return nil, errors.Errorf(
					"parsing enforcement level for %q: %q is not a valid enforcement level", k, val)
			}
			enforcementLevel = el
		case map[string]interface{}:
			el, err := extractEnforcementLevel(val)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing enforcement level for %q", k)
			}
			enforcementLevel = el
			if len(val) > 0 {
				properties = val
			}
		default:
			return nil, errors.Errorf("parsing %q: %v is not a valid value; must be a string or object", k, v)
		}

		// Don't bother including empty configs.
		if enforcementLevel == "" && len(properties) == 0 {
			continue
		}

		result[k] = plugin.AnalyzerPolicyConfig{
			EnforcementLevel: enforcementLevel,
			Properties:       properties,
		}
	}
	return result, nil
}

// extractEnforcementLevel looks for "enforcementLevel" in the map, and if so, validates that it is a valid value, and
// if so, deletes it from the map and returns it.
func extractEnforcementLevel(props map[string]interface{}) (apitype.EnforcementLevel, error) {
	contract.Assertf(props != nil, "props != nil")

	var enforcementLevel apitype.EnforcementLevel
	if unknown, ok := props["enforcementLevel"]; ok {
		enforcementLevelStr, isStr := unknown.(string)
		if !isStr {
			return "", errors.Errorf("%v is not a valid enforcement level; must be a string", unknown)
		}
		el := apitype.EnforcementLevel(enforcementLevelStr)
		if !el.IsValid() {
			return "", errors.Errorf("%q is not a valid enforcement level", enforcementLevelStr)
		}
		enforcementLevel = el
		// Remove enforcementLevel from the map.
		delete(props, "enforcementLevel")
	}
	return enforcementLevel, nil
}

// ValidatePolicyPackConfig validates the policy pack's configuration.
func validatePolicyPackConfig(
	policies []plugin.AnalyzerPolicyInfo, config map[string]plugin.AnalyzerPolicyConfig) ([]string, error) {
	contract.Assertf(config != nil, "contract != nil")
	var errors []string
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
			return nil, err
		}
		for _, validationError := range validationErrors {
			errors = append(errors, fmt.Sprintf("%s: %s", policy.Name, validationError))
		}
	}
	return errors, nil
}

// validatePolicyConfig validates an individual policy's configuration.
func validatePolicyConfig(schema plugin.AnalyzerPolicyConfigSchema, config map[string]interface{}) ([]string, error) {
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

// ValidatePolicyPackConfig validates a policy pack configuration against the specified config schema.
func ValidatePolicyPackConfig(schemaMap map[string]apitype.PolicyConfigSchema,
	config map[string]*json.RawMessage) (err error) {
	for property, schema := range schemaMap {
		schemaLoader := gojsonschema.NewGoLoader(schema)

		// If the config for this property is nil, we override it with an empty
		// json struct to ensure the config is not missing any required properties.
		propertyConfig := config[property]
		if propertyConfig == nil {
			temp := json.RawMessage([]byte(`{}`))
			propertyConfig = &temp
		}
		configLoader := gojsonschema.NewBytesLoader(*propertyConfig)
		result, err := gojsonschema.Validate(schemaLoader, configLoader)
		if err != nil {
			return errors.Wrap(err, "unable to validate schema")
		}

		// If the result is invalid, we need to gather the errors to return to the user.
		if !result.Valid() {
			resultErrs := make([]string, len(result.Errors()))
			for i, e := range result.Errors() {
				resultErrs[i] = e.Description()
			}
			msg := fmt.Sprintf("policy pack configuration is invalid: %s", strings.Join(resultErrs, ", "))
			return errors.New(msg)
		}
	}
	return err
}

func convertSchema(schema plugin.AnalyzerPolicyConfigSchema) plugin.JSONSchema {
	result := plugin.JSONSchema{}
	result["type"] = "object"
	if len(schema.Properties) > 0 {
		result["properties"] = schema.Properties
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	return result
}

// createConfigWithDefaults returns a new map filled-in with defaults from the policy metadata.
func createConfigWithDefaults(policies []plugin.AnalyzerPolicyInfo) map[string]plugin.AnalyzerPolicyConfig {
	result := make(map[string]plugin.AnalyzerPolicyConfig)

	// Prepare the resulting config with all defaults from the policy metadata.
	for _, policy := range policies {
		var props map[string]interface{}

		// Set default values from the schema.
		if policy.ConfigSchema != nil {
			for k, v := range policy.ConfigSchema.Properties {
				if val, ok := v["default"]; ok {
					if props == nil {
						props = make(map[string]interface{})
					}
					props[k] = val
				}
			}
		}

		result[policy.Name] = plugin.AnalyzerPolicyConfig{
			EnforcementLevel: policy.EnforcementLevel,
			Properties:       props,
		}
	}

	return result
}

// ReconcilePolicyPackConfig takes metadata about each policy containing default values and config schema, and
// reconciles this with the given config to produce a new config that has all default values filled-in and then sets
// configured values.
func ReconcilePolicyPackConfig(
	policies []plugin.AnalyzerPolicyInfo,
	initialConfig map[string]plugin.AnalyzerPolicyConfig,
	config map[string]plugin.AnalyzerPolicyConfig,
) (map[string]plugin.AnalyzerPolicyConfig, []string, error) {
	// Prepare the resulting config with all defaults from the policy metadata.
	result := createConfigWithDefaults(policies)
	contract.Assertf(result != nil, "result != nil")

	// Apply initial config supplied programmatically.
	if initialConfig != nil {
		result = applyConfig(result, initialConfig)
	}

	// Apply additional config from API or CLI.
	if config != nil {
		result = applyConfig(result, config)
	}

	// Validate the resulting config.
	validationErrors, err := validatePolicyPackConfig(policies, result)
	if err != nil {
		return nil, nil, err
	}
	if len(validationErrors) > 0 {
		return nil, validationErrors, nil
	}
	return result, nil, nil
}

func applyConfig(result map[string]plugin.AnalyzerPolicyConfig,
	configToApply map[string]plugin.AnalyzerPolicyConfig) map[string]plugin.AnalyzerPolicyConfig {
	// Apply anything that applies to "all" policies.
	if all, hasAll := configToApply["all"]; hasAll && all.EnforcementLevel.IsValid() {
		for k, v := range result {
			result[k] = plugin.AnalyzerPolicyConfig{
				EnforcementLevel: all.EnforcementLevel,
				Properties:       v.Properties,
			}
		}
	}
	// Apply policy level config.
	for policy, givenConfig := range configToApply {
		var enforcementLevel apitype.EnforcementLevel
		var properties map[string]interface{}

		if resultConfig, hasResultConfig := result[policy]; hasResultConfig {
			enforcementLevel = resultConfig.EnforcementLevel
			properties = resultConfig.Properties
		}

		if givenConfig.EnforcementLevel.IsValid() {
			enforcementLevel = givenConfig.EnforcementLevel
		}
		if len(givenConfig.Properties) > 0 && properties == nil {
			properties = make(map[string]interface{})
		}
		for k, v := range givenConfig.Properties {
			properties[k] = v
		}
		result[policy] = plugin.AnalyzerPolicyConfig{
			EnforcementLevel: enforcementLevel,
			Properties:       properties,
		}
	}
	return result
}
