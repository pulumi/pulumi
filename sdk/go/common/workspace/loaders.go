// Copyright 2016-2021, Pulumi Corporation.
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

package workspace

import (
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// readFileStripUTF8BOM wraps os.ReadFile and also strips the UTF-8 Byte-order Mark (BOM) if present.
func readFileStripUTF8BOM(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Strip UTF-8 BOM bytes if present to avoid problems with downstream parsing.
	// References:
	//   https://github.com/spkg/bom
	//   https://en.wikipedia.org/wiki/Byte_order_mark
	if len(b) >= 3 &&
		b[0] == 0xef &&
		b[1] == 0xbb &&
		b[2] == 0xbf {
		b = b[3:]
	}

	return b, nil
}

// Rewrite config values to make them namespaced. Using the project name as the default namespace
// for example:
//
//	config:
//	  instanceSize: t3.micro
//
// is valid configuration and will be rewritten in the form
//
//	config:
//	  {projectName}:instanceSize:t3.micro
func stackConfigNamespacedWithProject(project *Project, projectStack map[string]interface{}) map[string]interface{} {
	if project == nil {
		// return the original config if we don't have a project
		return projectStack
	}

	config, ok := projectStack["config"]
	if ok {
		configAsMap, isMap := config.(map[string]interface{})
		if isMap {
			modifiedConfig := make(map[string]interface{})
			for key, value := range configAsMap {
				if strings.Contains(key, ":") {
					// key is already namespaced
					// use it as is
					modifiedConfig[key] = value
				} else {
					namespacedKey := fmt.Sprintf("%s:%s", project.Name, key)
					modifiedConfig[namespacedKey] = value
				}
			}

			projectStack["config"] = modifiedConfig
			return projectStack
		}
	}

	return projectStack
}

// LoadProject reads a project definition from a file.
func LoadProject(path string) (*Project, error) {
	contract.Requiref(path != "", "path", "must not be empty")

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, fmt.Errorf("can not read '%s': %w", path, err)
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, fmt.Errorf("could not read '%s': %w", path, err)
	}

	return LoadProjectBytes(b, path, marshaller)
}

// LoadProjectBytes reads a project definition from a byte slice.
func LoadProjectBytes(b []byte, path string, marshaller encoding.Marshaler) (*Project, error) {
	var raw interface{}
	err := marshaller.Unmarshal(b, &raw)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal '%s': %w", path, err)
	}

	err = ValidateProject(raw)
	if err != nil {
		return nil, fmt.Errorf("could not validate '%s': %w", path, err)
	}

	// just before marshalling, we will rewrite the config values
	projectDef, err := SimplifyMarshalledProject(raw)
	if err != nil {
		return nil, err
	}

	projectDef, rewriteError := RewriteConfigPathIntoStackConfigDir(projectDef)
	if rewriteError != nil {
		return nil, rewriteError
	}

	projectDef = RewriteShorthandConfigValues(projectDef)
	modifiedProject, _ := marshaller.Marshal(projectDef)

	var project Project
	err = marshaller.Unmarshal(modifiedProject, &project)
	if err != nil {
		return nil, err
	}

	err = project.Validate()
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal '%s': %w", path, err)
	}

	project.raw = b
	return &project, nil
}

// LoadProjectStack reads a stack definition from a file.
func LoadProjectStack(project *Project, path string) (*ProjectStack, error) {
	contract.Requiref(path != "", "path", "must not be empty")

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if os.IsNotExist(err) {
		defaultProjectStack := ProjectStack{
			Config: make(config.Map),
		}
		return &defaultProjectStack, nil
	} else if err != nil {
		return nil, err
	}

	return LoadProjectStackBytes(project, b, path, marshaller)
}

func LoadProjectStackDeployment(path string) (*ProjectStackDeployment, error) {
	contract.Requiref(path != "", "path", "must not be empty")

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return LoadProjectStackDeploymentBytes(b, path, marshaller)
}

// LoadProjectStack reads a stack definition from a byte slice.
func LoadProjectStackBytes(
	project *Project,
	b []byte,
	path string,
	marshaller encoding.Marshaler,
) (*ProjectStack, error) {
	var projectStackRaw interface{}
	err := marshaller.Unmarshal(b, &projectStackRaw)
	if err != nil {
		return nil, err
	}

	if projectStackRaw == nil {
		// for example when reading an empty stack file
		defaultProjectStack := ProjectStack{
			Config: make(config.Map),
		}
		return &defaultProjectStack, nil
	}

	simplifiedStackForm, err := SimplifyMarshalledProject(projectStackRaw)
	if err != nil {
		return nil, err
	}

	// rewrite config values to make them namespaced
	// for example:
	//     config:
	//       instanceSize: t3.micro
	//
	// is valid configuration and will be rewritten in the form
	//
	//     config:
	//       {projectName}:instanceSize: t3.micro
	projectStackWithNamespacedConfig := stackConfigNamespacedWithProject(project, simplifiedStackForm)

	configObj, ok := simplifiedStackForm["config"]
	if ok {
		if configMap, ok := configObj.(map[string]interface{}); ok {
			checkForEmptyConfig(configMap)
		}
	}

	modifiedProjectStack, _ := marshaller.Marshal(projectStackWithNamespacedConfig)

	var projectStack ProjectStack
	err = marshaller.Unmarshal(modifiedProjectStack, &projectStack)
	if err != nil {
		return nil, err
	}

	if projectStack.Config == nil {
		projectStack.Config = make(config.Map)
	}

	projectStack.raw = b
	return &projectStack, nil
}

func LoadProjectStackDeploymentBytes(
	b []byte,
	path string,
	marshaller encoding.Marshaler,
) (*ProjectStackDeployment, error) {
	var projectStackDeployment ProjectStackDeployment
	err := marshaller.Unmarshal(b, &projectStackDeployment)
	if err != nil {
		return nil, err
	}

	return &projectStackDeployment, nil
}

// LoadPluginProject reads a plugin project definition from a file.
func LoadPluginProject(path string) (*PluginProject, error) {
	contract.Requiref(path != "", "path", "must not be empty")

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, err
	}

	var pluginProject PluginProject
	err = marshaller.Unmarshal(b, &pluginProject)
	if err != nil {
		return nil, err
	}

	err = pluginProject.Validate()
	if err != nil {
		return nil, err
	}

	return &pluginProject, nil
}

// LoadPolicyPack reads a policy pack definition from a file.
func LoadPolicyPack(path string) (*PolicyPackProject, error) {
	contract.Requiref(path != "", "path", "must not be empty")

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, err
	}

	var policyPackProject PolicyPackProject
	err = marshaller.Unmarshal(b, &policyPackProject)
	if err != nil {
		return nil, err
	}

	err = policyPackProject.Validate()
	if err != nil {
		return nil, err
	}

	return &policyPackProject, nil
}

// checkForEmptyConfig emits a warning for any config values that are `null` in
// the YAML/JSON. These are currently loaded into a `config.Value` that's
// interpreted as an empty string when we unmarshal into `ProjectStack.Config`,
// but we want to change this in the future to maintain nullness.
// Note that for object values, null is maintained inside objects already.
//
// ```yaml
// # somekey{1,2,3} are all `null` in YAML, but `""` in pulumi.
// somekey1:
// somekey2: null
// somekey3: ~
//
// # someobj.somekey{4,5,6} is `null` in both YAML and pulumi
// someobj:
//
//	somekey4:
//	somekey5: null
//	somekey6: ~
//
// ```
func checkForEmptyConfig(config map[string]interface{}) {
	keys := []string{}
	for k, v := range config {
		if v == nil {
			keys = append(keys, k)
		}
	}

	if len(keys) == 1 {
		cmdutil.Diag().Warningf(&diag.Diag{
			Message: fmt.Sprintf("No value for configuration key %q. This is currently treated as an empty string `\"\"`, "+
				"but will be treated as `null` in a future version of pulumi.", keys[0]),
		})
	} else if len(keys) > 1 {
		for i, k := range keys {
			keys[i] = `"` + k + `"`
		}
		joiner := ", "
		if len(keys) == 2 {
			joiner = " and "
		}
		cmdutil.Diag().Warningf(&diag.Diag{
			Message: fmt.Sprintf("No value for configuration keys %s. This is currently treated as an empty string `\"\"`, "+
				"but will be treated as `null` in a future version of pulumi.", strings.Join(keys, joiner)),
		})
	}
}
