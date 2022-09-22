package workspace

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func ValidateStackConfigAndApplyProjectConfig(
	stackName string,
	project *Project,
	stackConfig config.Map) error {
	for projectConfigKey, projectConfigType := range project.Config {
		key := config.MustMakeKey(string(project.Name), projectConfigKey)
		stackValue, found, _ := stackConfig.Get(key, true)
		hasDefault := projectConfigType.Default != nil
		if !found && !hasDefault {
			missingConfigError := fmt.Errorf(
				"Stack '%v' missing configuration value '%v'",
				stackName,
				projectConfigKey)
			return missingConfigError
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
			// found value on the stack level
			// retrieve it and validate it against
			// the config defined at the project level
			content, contentError := stackValue.MarshalValue()
			if contentError != nil {
				return contentError
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
		}
	}

	return nil
}
