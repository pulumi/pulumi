package workspace

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

func missingProjectConfigurationKeysError(missingProjectKeys []string, stackName string) error {
	valueOrValues := "value"
	if len(missingProjectKeys) > 1 {
		valueOrValues = "values"
	}

	isOrAre := "is"
	if len(missingProjectKeys) > 1 {
		isOrAre = "are"
	}

	return fmt.Errorf(
		"Stack '%v' uses configuration %v %v which %v not defined by the project configuration",
		stackName,
		valueOrValues,
		formatMissingKeys(missingProjectKeys),
		isOrAre)
}

type StackName = string
type ProjectConfigKey = string
type StackConfigValidator = func(StackName, ProjectConfigKey, ProjectConfigType, config.Value, config.Decrypter) error

func DefaultStackConfigValidator(
	stackName string,
	projectConfigKey string,
	projectConfigType ProjectConfigType,
	stackValue config.Value,
	dec config.Decrypter) error {
	// First check if the project says this should be secret, and if so that the stack value is
	// secure.
	if projectConfigType.Secret && !stackValue.Secure() {
		validationError := fmt.Errorf(
			"Stack '%v' with configuration key '%v' must be encrypted as it's secret",
			stackName,
			projectConfigKey)
		return validationError
	}

	value, err := stackValue.Value(dec)
	if err != nil {
		return err
	}
	// Content will be a JSON string if object is true, so marshal that back into an actual structure
	var content interface{} = value
	if stackValue.Object() {
		err = json.Unmarshal([]byte(value), &content)
		if err != nil {
			return err
		}
	}

	if !ValidateConfigValue(projectConfigType.Type, projectConfigType.Items, content) {
		typeName := InferFullTypeName(projectConfigType.Type, projectConfigType.Items)
		validationError := fmt.Errorf(
			"Stack '%v' with configuration key '%v' must be of type '%v'",
			stackName,
			projectConfigKey,
			typeName)

		return validationError
	}

	return nil
}

func ValidateStackConfigAndMergeProjectConfig(
	stackName string,
	project *Project,
	stackConfig config.Map,
	lazyDecrypter func() config.Decrypter,
	validate StackConfigValidator) error {

	if len(project.Config) > 0 {
		// only when the project defines config values, do we need to validate the stack config
		// for each stack config key, check if it is in the project config
		stackConfigKeysNotDefinedByProject := []string{}
		for key := range stackConfig {
			namespacedKey := fmt.Sprintf("%s:%s", key.Namespace(), key.Name())
			if key.Namespace() == string(project.Name) {
				// then the namespace is implied and can be omitted
				namespacedKey = key.Name()
			}

			if _, ok := project.Config[namespacedKey]; !ok {
				stackConfigKeysNotDefinedByProject = append(stackConfigKeysNotDefinedByProject, namespacedKey)
			}
		}

		if len(stackConfigKeysNotDefinedByProject) > 0 {
			return missingProjectConfigurationKeysError(stackConfigKeysNotDefinedByProject, stackName)
		}
	}

	var decrypter config.Decrypter
	missingConfigurationKeys := make([]string, 0)
	for projectConfigKey, projectConfigType := range project.Config {
		var key config.Key
		if strings.Contains(projectConfigKey, ":") {
			// key is already namespaced
			parsedKey, parseError := config.ParseKey(projectConfigKey)
			if parseError != nil {
				return parseError
			}

			key = parsedKey
		} else {
			// key is not namespaced
			// use the project as namespace
			key = config.MustMakeKey(string(project.Name), projectConfigKey)
		}

		stackValue, found, err := stackConfig.Get(key, true)
		if err != nil {
			return fmt.Errorf("Error while getting stack config value for key '%v': %v", key.String(), err)
		}

		hasDefault := projectConfigType.Default != nil
		if !found && !hasDefault {
			// add it to the list to collect all missing configuration keys,
			// then return them as a single error
			missingConfigurationKeys = append(missingConfigurationKeys, projectConfigKey)
		} else if !found && hasDefault {
			// not found at the stack level
			// but has a default value at the project level
			// assign the value to the stack
			var configValue config.Value

			if projectConfigType.Type == "array" {
				// for array types, JSON-ify the default value
				configValueJSON, jsonError := json.Marshal(projectConfigType.Default)
				if jsonError != nil {
					return jsonError
				}
				configValue = config.NewObjectValue(string(configValueJSON))

			} else {
				// for primitive types
				// pass the values as is
				configValueContent := fmt.Sprintf("%v", projectConfigType.Default)
				configValue = config.NewValue(configValueContent)
			}

			setError := stackConfig.Set(key, configValue, true)
			if setError != nil {
				return setError
			}
		} else {
			// Validate stack level value against the config defined at the project level
			if validate != nil {
				// we have a validator
				if decrypter == nil && lazyDecrypter != nil {
					// initialize the decrypter once
					decrypter = lazyDecrypter()
				}

				if decrypter != nil {
					validationError := validate(stackName, projectConfigKey, projectConfigType, stackValue, decrypter)
					if validationError != nil {
						return validationError
					}
				}
			}
		}
	}

	if len(missingConfigurationKeys) > 0 {
		// there are missing configuration keys in the stack
		// return them as a single error.
		return missingStackConfigurationKeysError(missingConfigurationKeys, stackName)
	}

	return nil
}

func ValidateStackConfigAndApplyProjectConfig(
	stackName string,
	project *Project,
	stackConfig config.Map,
	dec config.Decrypter) error {
	decrypter := func() config.Decrypter {
		return dec
	}

	return ValidateStackConfigAndMergeProjectConfig(
		stackName, project, stackConfig, decrypter, DefaultStackConfigValidator)
}

// ApplyConfigDefaults applies the default values for the project configuration onto the stack configuration
// without validating the contents of stack config values.
// This is because sometimes during pulumi config ls and pulumi config get, if users are
// using PassphraseDecrypter, we don't want to always prompt for the values when not necessary
func ApplyProjectConfig(stackName string, project *Project, stackConfig config.Map) error {
	return ValidateStackConfigAndMergeProjectConfig(stackName, project, stackConfig, nil, nil)
}
