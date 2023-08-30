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

type (
	StackName        = string
	ProjectConfigKey = string
)

func validateStackConfigValue(
	stackName string,
	projectConfigKey string,
	projectConfigType ProjectConfigType,
	stackValue config.Value,
	dec config.Decrypter,
) error {
	if dec == nil {
		return nil
	}

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
	stackName string,
	project *Project,
	stackConfig config.Map,
	decrypter config.Decrypter,
	validate bool,
) error {
	missingConfigurationKeys := make([]string, 0)
	projectName := project.Name.String()

	keys := make([]string, 0, len(project.Config))
	for k := range project.Config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, projectConfigKey := range keys {
		projectConfigType := project.Config[projectConfigKey]

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
			// use the project as default namespace
			key = config.MustMakeKey(projectName, projectConfigKey)
		}

		stackValue, foundOnStack, err := stackConfig.Get(key, true)
		if err != nil {
			return fmt.Errorf("Error while getting stack config value for key '%v': %v", key.String(), err)
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

		// Validate stack level value against the config defined at the project level
		if validate && projectConfigType.IsExplicitlyTyped() {
			err := validateStackConfigValue(stackName, projectConfigKey, projectConfigType, stackValue, decrypter)
			if err != nil {
				return err
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
	decrypter config.Decrypter,
) error {
	return mergeConfig(stackName, project, stackConfig, decrypter, true)
}

// ApplyConfigDefaults applies the default values for the project configuration onto the stack configuration
// without validating the contents of stack config values.
// This is because sometimes during pulumi config ls and pulumi config get, if users are
// using PassphraseDecrypter, we don't want to always prompt for the values when not necessary
func ApplyProjectConfig(stackName string, project *Project, stackConfig config.Map) error {
	return mergeConfig(stackName, project, stackConfig, nil, false)
}
