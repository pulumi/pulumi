// Copyright 2022-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func formatMissingKeys(missingKeys []string) string {
	if len(missingKeys) == 1 {
		return fmt.Sprintf("'%v'", missingKeys[0])
	}

	sort.Strings(missingKeys)

	formattedMissingKeys := ""
	for index, key := range missingKeys {
		// if last index, then use and before the key
		if index == len(missingKeys)-1 {
			formattedMissingKeys += fmt.Sprintf("and '%s'", key)
		} else if index == len(missingKeys)-2 {
			// no comma before the last key
			formattedMissingKeys += fmt.Sprintf("'%s' ", key)
		} else {
			formattedMissingKeys += fmt.Sprintf("'%s', ", key)
		}
	}

	return formattedMissingKeys
}

func missingStackConfigurationKeysError(missingKeys []string, stackName string) error {
	valueOrValues := "value"
	if len(missingKeys) > 1 {
		valueOrValues = "values"
	}

	return fmt.Errorf(
		"Stack '%v' is missing configuration %v %v",
		stackName,
		valueOrValues,
		formatMissingKeys(missingKeys))
}

type (
	StackName        = string
	ProjectConfigKey = string
)

func validateStackConfigValues(
	stackName string,
	project *Project,
	stackConfig config.Map,
	dec config.Decrypter,
) error {
	if dec == nil {
		return nil
	}

	// TODO: Better use stackConfig.AsDecryptedPropertyMap here and validate against that
	decryptedConfig, err := stackConfig.Decrypt(dec)
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(project.Config))
	for k := range project.Config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, projectConfigKey := range keys {
		projectConfigType := project.Config[projectConfigKey]

		key, err := config.ParseConfigKey(project.Name.String(), projectConfigKey)
		if err != nil {
			return err
		}

		stackValue, _, err := stackConfig.Get(key, true)
		if err != nil {
			return fmt.Errorf("getting stack config value for key '%v': %w", key.String(), err)
		}

		if projectConfigType.IsExplicitlyTyped() {
			decryptedValue := decryptedConfig[key]
			err := validateStackConfigValue(stackName, projectConfigKey, projectConfigType, stackValue, decryptedValue)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func validateStackConfigValue(
	stackName string,
	projectConfigKey string,
	projectConfigType ProjectConfigType,
	stackValue config.Value,
	decryptedValue string,
) error {
	// First check if the project says this should be secret, and if so that the stack value is
	// secure.
	if projectConfigType.Secret && !stackValue.Secure() {
		validationError := fmt.Errorf(
			"Stack '%v' with configuration key '%v' must be encrypted as it's secret",
			stackName,
			projectConfigKey)
		return validationError
	}

	// Content will be a JSON string if object is true, so marshal that back into an actual structure
	var content interface{} = decryptedValue
	if stackValue.Object() {
		err := json.Unmarshal([]byte(decryptedValue), &content)
		if err != nil {
			return err
		}
	}

	if !ValidateConfigValue(*projectConfigType.Type, projectConfigType.Items, content) {
		typeName := InferFullTypeName(*projectConfigType.Type, projectConfigType.Items)
		validationError := fmt.Errorf(
			"Stack '%v' with configuration key '%v' must be of type '%v'",
			stackName,
			projectConfigKey,
			typeName)

		return validationError
	}

	return nil
}

func createConfigValue(rawValue interface{}) (config.Value, error) {
	if isPrimitiveValue(rawValue) {
		configValueContent := fmt.Sprintf("%v", rawValue)
		return config.NewValue(configValueContent), nil
	}
	value, err := SimplifyMarshalledValue(rawValue)
	if err != nil {
		return config.Value{}, err
	}
	configValueJSON, jsonError := json.Marshal(value)
	if jsonError != nil {
		return config.Value{}, jsonError
	}
	return config.NewObjectValue(string(configValueJSON)), nil
}

func mergeConfig(
	ctx context.Context,
	stackName string,
	project *Project,
	stackEnv esc.Value,
	stackConfig config.Map,
	encrypter config.Encrypter,
	decrypter config.Decrypter,
	validate bool,
) error {
	projectName := project.Name.String()

	envConfig, err := config.ToConfigMap(ctx, stackEnv, projectName, encrypter)
	if err != nil {
		return err
	}

	// First merge the stack environment and the stack config together.
	for key, envValue := range envConfig {
		stackValue, foundOnStack, err := stackConfig.Get(key, false)
		if err != nil {
			return fmt.Errorf("getting stack config value for key '%v': %w", key.String(), err)
		}

		if !foundOnStack {
			err = stackConfig.Set(key, envValue, false)
		} else {
			merged, mergeErr := stackValue.Merge(envValue)
			if mergeErr != nil {
				return fmt.Errorf("merging environment config for key '%v': %w", key.String(), err)
			}
			err = stackConfig.Set(key, merged, false)
		}
		if err != nil {
			return fmt.Errorf("setting merged config value for key '%v': %w", key.String(), err)
		}
	}

	missingConfigurationKeys := make([]string, 0)
	keys := make([]string, 0, len(project.Config))
	for k := range project.Config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Next validate the merged config and merge in the project config.
	for _, projectConfigKey := range keys {
		projectConfigType := project.Config[projectConfigKey]

		key, err := config.ParseConfigKey(projectName, projectConfigKey)
		if err != nil {
			return err
		}

		_, foundOnStack, err := stackConfig.Get(key, true)
		if err != nil {
			return fmt.Errorf("getting stack config value for key '%v': %w", key.String(), err)
		}

		hasDefault := projectConfigType.Default != nil
		hasValue := projectConfigType.Value != nil

		if !foundOnStack && !hasValue && !hasDefault && key.Namespace() == projectName {
			// add it to the list of missing project configuration keys in the stack
			// which are required by the project
			// then return them as a single error
			missingConfigurationKeys = append(missingConfigurationKeys, projectConfigKey)
			continue
		}

		if !foundOnStack && (hasValue || hasDefault) {
			// either value or default value is provided
			var value interface{}
			if hasValue {
				value = projectConfigType.Value
			}
			if hasDefault {
				value = projectConfigType.Default
			}
			// it is not found on the stack we are currently validating / merging values with
			// then we assign the value to that stack whatever that value is
			configValue, err := createConfigValue(value)
			if err != nil {
				return err
			}
			setError := stackConfig.Set(key, configValue, true)
			if setError != nil {
				return setError
			}

			continue
		}
	}

	if len(missingConfigurationKeys) > 0 {
		// there are missing configuration keys in the stack
		// return them as a single error.
		return missingStackConfigurationKeysError(missingConfigurationKeys, stackName)
	}

	// Validate stack level values against the config defined at the project level
	if validate {
		err := validateStackConfigValues(stackName, project, stackConfig, decrypter)
		if err != nil {
			return err
		}
	}

	return nil
}

func ValidateStackConfigAndApplyProjectConfig(
	ctx context.Context,
	stackName string,
	project *Project,
	stackEnv esc.Value,
	stackConfig config.Map,
	encrypter config.Encrypter,
	decrypter config.Decrypter,
) error {
	return mergeConfig(ctx, stackName, project, stackEnv, stackConfig, encrypter, decrypter, true)
}

// ApplyConfigDefaults applies the default values for the project configuration onto the stack configuration
// without validating the contents of stack config values.
// This is because sometimes during pulumi config ls and pulumi config get, if users are
// using PassphraseDecrypter, we don't want to always prompt for the values when not necessary
func ApplyProjectConfig(
	ctx context.Context,
	stackName string,
	project *Project,
	stackEnv esc.Value,
	stackConfig config.Map,
	encrypter config.Encrypter,
) error {
	return mergeConfig(ctx, stackName, project, stackEnv, stackConfig, encrypter, nil, false)
}
