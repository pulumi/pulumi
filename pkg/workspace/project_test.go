// Copyright 2016-2023, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deleteFile(t *testing.T, file *os.File) {
	if file != nil {
		err := os.Remove(file.Name())
		assert.NoError(t, err, "Error while deleting file")
	}
}

func loadProjectFromText(t *testing.T, content string) (*workspace.Project, error) {
	tmp, err := os.CreateTemp("", "*.yaml")
	assert.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	assert.NoError(t, err)
	defer deleteFile(t, tmp)
	return workspace.LoadProject(path)
}

func loadProjectStackFromText(
	t *testing.T,
	project *workspace.Project,
	content string,
) (*workspace.ProjectStack, error) {
	tmp, err := os.CreateTemp("", "*.yaml")
	assert.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	assert.NoError(t, err)
	defer deleteFile(t, tmp)
	return workspace.LoadProjectStack(project, path)
}

func getConfigValue(t *testing.T, stackConfig config.Map, key string) string {
	parsedKey, err := config.ParseKey(key)
	assert.NoErrorf(t, err, "There should be no error parsing the config key '%v'", key)
	configValue, foundValue := stackConfig[parsedKey]
	assert.Truef(t, foundValue, "Couldn't find a value for config key %v", key)
	value, valueError := configValue.Value(config.NopDecrypter)
	assert.NoErrorf(t, valueError, "Error while getting the value for key %v", key)
	return value
}

func getConfigValueUnmarshalled(t *testing.T, stackConfig config.Map, key string) interface{} {
	parsedKey, err := config.ParseKey(key)
	assert.NoErrorf(t, err, "There should be no error parsing the config key '%v'", key)
	configValue, foundValue := stackConfig[parsedKey]
	assert.Truef(t, foundValue, "Couldn't find a value for config key %v", key)
	valueJSON, valueError := configValue.Value(config.NopDecrypter)
	assert.NoErrorf(t, valueError, "Error while getting the value for key %v", key)
	var value interface{}
	err = json.Unmarshal([]byte(valueJSON), &value)
	assert.NoErrorf(t, err, "Error while unmarshalling value for key %v", key)
	return value
}

func TestStackConfigIsInheritedFromProjectConfig(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  instanceSize: t3.micro
  instanceCount: 20
  protect: true`

	projectStackYaml := `
config:
  test:instanceSize: t4.large`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")

	assert.Equal(t, 3, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "t4.large", getConfigValue(t, stack.Config, "test:instanceSize"))
	// instanceCount and protect are inherited from the project
	assert.Equal(t, "20", getConfigValue(t, stack.Config, "test:instanceCount"))
	assert.Equal(t, "true", getConfigValue(t, stack.Config, "test:protect"))
}

func TestNamespacedConfigValuesAreInheritedCorrectly(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  aws:region: us-west-1
  pulumi:disable-default-providers: ["*"]
  instanceSize: t3.micro`

	projectStackYaml := `
config:
  test:instanceSize: t4.large`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")
	assert.Equal(t, 3, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "t4.large", getConfigValue(t, stack.Config, "test:instanceSize"))
	// aws:region is namespaced and is inherited from the project
	assert.Equal(t, "us-west-1", getConfigValue(t, stack.Config, "aws:region"))
	assert.Equal(t, "[\"*\"]", getConfigValue(t, stack.Config, "pulumi:disable-default-providers"))
	assert.Equal(t, []interface{}{"*"}, getConfigValueUnmarshalled(t, stack.Config, "pulumi:disable-default-providers"))
}

func TestLoadingStackConfigWithoutNamespacingTheProject(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  aws:region: us-west-1
  instanceSize: t3.micro`

	projectStackYaml := `
config:
  instanceSize: t4.large`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")

	assert.Equal(t, 2, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "t4.large", getConfigValue(t, stack.Config, "test:instanceSize"))
	// aws:region is namespaced and is inherited from the project
	assert.Equal(t, "us-west-1", getConfigValue(t, stack.Config, "aws:region"))
}

func TestUntypedProjectConfigValuesAreNotValidated(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  instanceSize: t3.micro
  aws:region: us-west-1`

	projectStackYaml := `
config:
  instanceSize: 9999
  aws:region: 42`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")
	assert.Equal(t, 2, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "9999", getConfigValue(t, stack.Config, "test:instanceSize"))
	assert.Equal(t, "42", getConfigValue(t, stack.Config, "aws:region"))
}

func TestUntypedProjectConfigValuesWithOnlyDefaultOrOnlyValue(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  instanceSize:
    default: t3.micro
  region:
    value: us-west-1`

	projectStackYaml := `
config:
  aws:answer: 42`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")
	assert.Equal(t, 3, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "t3.micro", getConfigValue(t, stack.Config, "test:instanceSize"))
	assert.Equal(t, "us-west-1", getConfigValue(t, stack.Config, "test:region"))
	assert.Equal(t, "42", getConfigValue(t, stack.Config, "aws:answer"))
}

func TestUntypedStackConfigValuesDoNeedProjectDeclaration(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  createVpc: true`

	projectStackYaml := `
config:
  instanceSize: 42`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")
	assert.Equal(t, 2, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "42", getConfigValue(t, stack.Config, "test:instanceSize"))
	assert.Equal(t, "true", getConfigValue(t, stack.Config, "test:createVpc"))
}

func TestUntypedProjectConfigObjectValuesPassedDownToStack(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  instanceSize:
    value:
      hello: world
  aws:config:
    value:
      region: us-west-1`

	projectStackYaml := `
config:
  aws:whatever: 42`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "Config override should be valid")
	assert.Equal(t, 3, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "{\"hello\":\"world\"}", getConfigValue(t, stack.Config, "test:instanceSize"))
	assert.Equal(t, "{\"region\":\"us-west-1\"}", getConfigValue(t, stack.Config, "aws:config"))
	assert.Equal(t, "42", getConfigValue(t, stack.Config, "aws:whatever"))
}

func TestStackConfigErrorsWhenStackValueIsNotCorrectlyTyped(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  values:
    type: array
    items:
      type: string
    default: [value]`

	projectStackYaml := `
config:
  test:values: someValue
`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.ErrorContains(t, configError, "Stack 'dev' with configuration key 'values' must be of type 'array<string>'")
}

func TestStackConfigIntegerTypeIsCorrectlyValidated(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  importantNumber:
    type: integer
`

	projectStackYamlValid := `
config:
  test:importantNumber: 20
`

	projectStackYamlInvalid := `
config:
  test:importantNumber: hello
`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYamlValid)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NoError(t, configError, "there should no config type error")

	invalidStackConfig, stackError := loadProjectStackFromText(t, project, projectStackYamlInvalid)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError = ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		invalidStackConfig.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t,
		configError.Error(),
		"Stack 'dev' with configuration key 'importantNumber' must be of type 'integer'")
}

func TestStackConfigErrorsWhenMissingStackValueForConfigTypeWithNoDefault(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  values:
    type: array
    items:
      type: string`

	projectStackYaml := ``

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.ErrorContains(t, configError, "Stack 'dev' is missing configuration value 'values'")
}

func TestStackConfigErrorsWhenMissingTwoStackValueForConfigTypeWithNoDefault(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  another:
    type: string
  values:
    type: array
    items:
      type: string`

	projectStackYaml := ``

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.ErrorContains(t, configError, "Stack 'dev' is missing configuration values 'another' and 'values'")
}

func TestStackConfigErrorsWhenMissingMultipleStackValueForConfigTypeWithNoDefault(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  hello:
    type: integer
  values:
    type: array
    items:
      type: string
  world:
    type: string`

	projectStackYaml := ``

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.ErrorContains(t, configError, "Stack 'dev' is missing configuration values 'hello', 'values' and 'world'")
}

func TestStackConfigDoesNotErrorWhenProjectHasNotDefinedConfig(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet`

	projectStackYaml := `
config:
  hello: 21
  world: 42
  another: 42`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYaml)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.Nil(t, configError, "there should not be a config type error")
}

func TestStackConfigSecretIsCorrectlyValidated(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  importantNumber:
    type: integer
    secret: true
`

	crypter := config.Base64Crypter
	encryptedValue, err := crypter.EncryptValue(context.Background(), "20")
	assert.NoError(t, err)

	projectStackYamlValid := fmt.Sprintf(`
config:
  test:importantNumber:
    secure: %s
`, encryptedValue)

	projectStackYamlInvalid := `
config:
  test:importantNumber: 20
`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, projectStackYamlValid)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		stack.Config,
		crypter,
		crypter)
	assert.NoError(t, configError, "there should no config type error")

	invalidStackConfig, stackError := loadProjectStackFromText(t, project, projectStackYamlInvalid)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError = ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		esc.Value{},
		invalidStackConfig.Config,
		crypter,
		crypter)
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t,
		configError.Error(),
		"Stack 'dev' with configuration key 'importantNumber' must be encrypted as it's secret")
}

//nolint:lll
func TestEnvironmentMerge(t *testing.T) {
	t.Parallel()

	projectYAML := `
name: test
runtime: nodejs`

	stackYAML := `
config:
  test:boolean: true
  test:number: 42
  test:string: foo
  test:array:
    - first
    - second
  test:object:
    foo: bar
    object:
      baz: 42`

	env := esc.NewValue(map[string]esc.Value{
		"test:boolean": esc.NewValue(false),
		"test:number":  esc.NewValue(json.Number("42")),
		"test:string":  esc.NewValue("esc"),
		"test:array":   esc.NewValue([]esc.Value{esc.NewValue("second"), esc.NewValue("first")}),
		"test:secret":  esc.NewSecret("hunter2"),
		"test:object": esc.NewValue(map[string]esc.Value{
			"boolean": esc.NewValue(true),
			"number":  esc.NewValue(json.Number("42")),
			"string":  esc.NewValue("esc"),
			"array":   esc.NewValue([]esc.Value{esc.NewValue("first"), esc.NewValue("second")}),
			"object":  esc.NewValue(map[string]esc.Value{"foo": esc.NewValue("bar")}),
			"foo":     esc.NewValue("qux"),
		}),
	})

	project, projectError := loadProjectFromText(t, projectYAML)
	require.NoError(t, projectError, "Shold be able to load the project")
	stack, stackError := loadProjectStackFromText(t, project, stackYAML)
	require.NoError(t, stackError, "Should be able to read the stack")

	configError := ValidateStackConfigAndApplyProjectConfig(
		"dev",
		project,
		env,
		stack.Config,
		config.Base64Crypter,
		config.Base64Crypter)
	require.NoError(t, configError, "there should not be a config type error")

	secureKeys := stack.Config.SecureKeys()
	assert.Equal(t, []config.Key{config.MustMakeKey("test", "secret")}, secureKeys)

	m, err := stack.Config.Decrypt(config.Base64Crypter)
	require.NoError(t, err)

	expected := map[config.Key]string{
		config.MustMakeKey("test", "array"):   "[\"first\",\"second\"]",
		config.MustMakeKey("test", "boolean"): "true",
		config.MustMakeKey("test", "number"):  "42",
		config.MustMakeKey("test", "object"):  "{\"array\":[\"first\",\"second\"],\"boolean\":true,\"foo\":\"bar\",\"number\":42,\"object\":{\"baz\":42,\"foo\":\"bar\"},\"string\":\"esc\"}",
		config.MustMakeKey("test", "string"):  "foo",
		config.MustMakeKey("test", "secret"):  "hunter2",
	}
	assert.Equal(t, expected, m)
}
