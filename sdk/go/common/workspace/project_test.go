package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestProjectRuntimeInfoRoundtripYAML(t *testing.T) {
	t.Parallel()

	doTest := func(marshal func(interface{}) ([]byte, error), unmarshal func([]byte, interface{}) error) {
		ri := NewProjectRuntimeInfo("nodejs", nil)
		byts, err := marshal(ri)
		assert.NoError(t, err)

		var riRountrip ProjectRuntimeInfo
		err = unmarshal(byts, &riRountrip)
		assert.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.Nil(t, riRountrip.Options())

		ri = NewProjectRuntimeInfo("nodejs", map[string]interface{}{
			"typescript":   true,
			"stringOption": "hello",
		})
		byts, err = marshal(ri)
		assert.NoError(t, err)
		err = unmarshal(byts, &riRountrip)
		assert.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.Equal(t, true, riRountrip.Options()["typescript"])
		assert.Equal(t, "hello", riRountrip.Options()["stringOption"])
	}

	doTest(yaml.Marshal, yaml.Unmarshal)
	doTest(json.Marshal, json.Unmarshal)
}

func TestProjectValidationForNameAndRuntime(t *testing.T) {
	t.Parallel()
	var err error

	// Test lack of name
	proj := Project{}
	err = proj.Validate()
	assert.Error(t, err)
	assert.Equal(t, "project is missing a 'name' attribute", err.Error())
	// Test lack of runtime
	proj.Name = "a project"
	err = proj.Validate()
	assert.Error(t, err)
	assert.Equal(t, "project is missing a 'runtime' attribute", err.Error())

	// Test success
	proj.Runtime = NewProjectRuntimeInfo("test", nil)
	err = proj.Validate()
	assert.NoError(t, err)
}

func TestProjectValidationFailsForIncorrectDefaultValueType(t *testing.T) {
	t.Parallel()
	project := Project{Name: "test", Runtime: NewProjectRuntimeInfo("dotnet", nil)}
	invalidConfig := make(map[string]ProjectConfigType)
	integerType := "integer"
	invalidConfig["instanceSize"] = ProjectConfigType{
		Type:    &integerType,
		Items:   nil,
		Default: "hello",
	}

	project.Config = invalidConfig
	err := project.Validate()
	assert.Contains(t,
		err.Error(),
		"The default value specified for configuration key 'instanceSize' is not of the expected type 'integer'")

	invalidValues := make([]interface{}, 0)
	invalidValues = append(invalidValues, "hello")
	// default value here has type array<string>
	// config type specified is array<array<string>>
	// should fail!
	arrayType := "array"
	invalidConfigWithArray := make(map[string]ProjectConfigType)
	invalidConfigWithArray["values"] = ProjectConfigType{
		Type: &arrayType,
		Items: &ProjectConfigItemsType{
			Type: "array",
			Items: &ProjectConfigItemsType{
				Type: "string",
			},
		},
		Default: invalidValues,
	}
	project.Config = invalidConfigWithArray
	err = project.Validate()
	assert.Error(t, err, "There is a validation error")
	assert.Contains(t,
		err.Error(),
		"The default value specified for configuration key 'values' is not of the expected type 'array<array<string>>'")
}

func TestProjectValidationSucceedsForCorrectDefaultValueType(t *testing.T) {
	t.Parallel()
	project := Project{Name: "test", Runtime: NewProjectRuntimeInfo("dotnet", nil)}
	integerType := "integer"
	validConfig := make(map[string]ProjectConfigType)
	validConfig["instanceSize"] = ProjectConfigType{
		Type:    &integerType,
		Items:   nil,
		Default: 1,
	}

	project.Config = validConfig
	err := project.Validate()
	assert.NoError(t, err, "There should be no validation error")

	// validValues = ["hello"]
	validValues := make([]interface{}, 0)
	validValues = append(validValues, "hello")
	// validValuesArray = [["hello"]]
	validValuesArray := make([]interface{}, 0)
	validValuesArray = append(validValuesArray, validValues)

	// default value here has type array<array<string>>
	// config type specified is also array<array<string>>
	// should succeed
	arrayType := "array"
	validConfigWithArray := make(map[string]ProjectConfigType)
	validConfigWithArray["values"] = ProjectConfigType{
		Type: &arrayType,
		Items: &ProjectConfigItemsType{
			Type: "array",
			Items: &ProjectConfigItemsType{
				Type: "string",
			},
		},
		Default: validValuesArray,
	}
	project.Config = validConfigWithArray
	err = project.Validate()
	assert.NoError(t, err, "There should be no validation error")
}

func TestProjectLoadJSON(t *testing.T) {
	t.Parallel()

	writeAndLoad := func(str string) (*Project, error) {
		tmp, err := os.CreateTemp("", "*.json")
		assert.NoError(t, err)
		path := tmp.Name()
		err = os.WriteFile(path, []byte(str), 0o600)
		assert.NoError(t, err)
		return LoadProject(path)
	}

	// Test wrong type
	_, err := writeAndLoad("\"hello  \"")
	assert.Contains(t, err.Error(), "expected project to be an object, was 'string'")

	// Test lack of name
	_, err = writeAndLoad("{}")
	assert.Contains(t, err.Error(), "project is missing a 'name' attribute")

	// Test bad name
	_, err = writeAndLoad("{\"name\": \"\"}")
	assert.Contains(t, err.Error(), "project is missing a non-empty string 'name' attribute")

	// Test missing runtime
	_, err = writeAndLoad("{\"name\": \"project\"}")
	assert.Contains(t, err.Error(), "project is missing a 'runtime' attribute")

	// Test other schema errors
	_, err = writeAndLoad("{\"name\": \"project\", \"runtime\": 4}")
	// These can vary in order, so contains not equals check
	expected := []string{
		"3 errors occurred:",
		"* #/runtime: oneOf failed",
		"* #/runtime: expected string, but got number",
		"* #/runtime: expected object, but got number",
	}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	_, err = writeAndLoad("{\"name\": \"project\", \"runtime\": \"test\", \"backend\": 4, \"main\": {}}")
	expected = []string{
		"2 errors occurred:",
		"* #/main: expected string or null, but got object",
		"* #/backend: expected object or null, but got number",
	}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	// Test success
	proj, err := writeAndLoad("{\"name\": \"project\", \"runtime\": \"test\"}")
	assert.NoError(t, err)
	assert.Equal(t, tokens.PackageName("project"), proj.Name)
	assert.Equal(t, "test", proj.Runtime.Name())

	// Test null optionals should work
	proj, err = writeAndLoad("{\"name\": \"project\", \"runtime\": \"test\", " +
		"\"description\": null, \"main\": null, \"backend\": null}")
	assert.NoError(t, err)
	assert.Nil(t, proj.Description)
	assert.Equal(t, "", proj.Main)
}

func deleteFile(t *testing.T, file *os.File) {
	if file != nil {
		err := os.Remove(file.Name())
		assert.NoError(t, err, "Error while deleting file")
	}
}

func loadProjectFromText(t *testing.T, content string) (*Project, error) {
	tmp, err := os.CreateTemp("", "*.yaml")
	assert.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	assert.NoError(t, err)
	defer deleteFile(t, tmp)
	return LoadProject(path)
}

func loadProjectStackFromText(t *testing.T, project *Project, content string) (*ProjectStack, error) {
	tmp, err := os.CreateTemp("", "*.yaml")
	assert.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	assert.NoError(t, err)
	defer deleteFile(t, tmp)
	return LoadProjectStack(project, path)
}

func TestProjectLoadsConfigSchemas(t *testing.T) {
	t.Parallel()
	projectContent := `
name: test
runtime: dotnet
config:
  integerSchemaFull:
    type: integer
    description: a very important value
    default: 1
  integerSchemaSimple: 20
  textSchemaFull:
    type: string
    default: t3.micro
  textSchemaSimple: t4.large
  booleanSchemaFull:
    type: boolean
    default: true
  booleanSchemaSimple: false
  simpleArrayOfStrings:
    type: array
    items:
      type: string
    default: [hello]
  arrayOfArrays:
    type: array
    items:
      type: array
      items:
        type: string
  secretString:
    type: string
    secret: true
  `

	project, err := loadProjectFromText(t, projectContent)
	assert.NoError(t, err, "Should be able to load the project")
	assert.Equal(t, 9, len(project.Config), "There are 9 config type definition")
	// full integer config schema
	integerSchemFull, ok := project.Config["integerSchemaFull"]
	assert.True(t, ok, "should be able to read integerSchemaFull")
	assert.Equal(t, "integer", integerSchemFull.TypeName())
	assert.Equal(t, "a very important value", integerSchemFull.Description)
	assert.Equal(t, 1, integerSchemFull.Default)
	assert.False(t, integerSchemFull.Secret)
	assert.Nil(t, integerSchemFull.Items, "Primtive config type doesn't have an items type")

	integerSchemaSimple, ok := project.Config["integerSchemaSimple"]
	assert.True(t, ok, "should be able to read integerSchemaSimple")
	assert.Equal(t, "", integerSchemaSimple.TypeName(), "not explicitly typed")
	assert.False(t, integerSchemaSimple.IsExplicitlyTyped())
	assert.False(t, integerSchemaSimple.Secret)
	assert.Equal(t, 20, integerSchemaSimple.Default, "Default integer value is parsed correctly")

	textSchemaFull, ok := project.Config["textSchemaFull"]
	assert.True(t, ok, "should be able to read textSchemaFull")
	assert.Equal(t, "string", textSchemaFull.TypeName())
	assert.False(t, textSchemaFull.Secret)
	assert.Equal(t, "t3.micro", textSchemaFull.Default)
	assert.Equal(t, "", textSchemaFull.Description)

	textSchemaSimple, ok := project.Config["textSchemaSimple"]
	assert.True(t, ok, "should be able to read textSchemaSimple")
	assert.Equal(t, "", textSchemaSimple.TypeName(), "not explicitly typed")
	assert.False(t, textSchemaSimple.IsExplicitlyTyped())
	assert.False(t, textSchemaSimple.Secret)
	assert.Equal(t, "t4.large", textSchemaSimple.Default)

	booleanSchemaFull, ok := project.Config["booleanSchemaFull"]
	assert.True(t, ok, "should be able to read booleanSchemaFull")
	assert.Equal(t, "boolean", booleanSchemaFull.TypeName())
	assert.False(t, booleanSchemaFull.Secret)
	assert.Equal(t, true, booleanSchemaFull.Default)

	booleanSchemaSimple, ok := project.Config["booleanSchemaSimple"]
	assert.True(t, ok, "should be able to read booleanSchemaSimple")
	assert.Equal(t, "", booleanSchemaSimple.TypeName(), "not explicitly typed")
	assert.False(t, booleanSchemaSimple.IsExplicitlyTyped())
	assert.False(t, booleanSchemaSimple.Secret)
	assert.Equal(t, false, booleanSchemaSimple.Default)

	simpleArrayOfStrings, ok := project.Config["simpleArrayOfStrings"]
	assert.True(t, ok, "should be able to read simpleArrayOfStrings")
	assert.Equal(t, "array", simpleArrayOfStrings.TypeName())
	assert.False(t, simpleArrayOfStrings.Secret)
	assert.NotNil(t, simpleArrayOfStrings.Items)
	assert.Equal(t, "string", simpleArrayOfStrings.Items.Type)
	arrayValues := simpleArrayOfStrings.Default.([]interface{})
	assert.Equal(t, "hello", arrayValues[0])

	arrayOfArrays, ok := project.Config["arrayOfArrays"]
	assert.True(t, ok, "should be able to read arrayOfArrays")
	assert.Equal(t, "array", arrayOfArrays.TypeName())
	assert.False(t, arrayOfArrays.Secret)
	assert.NotNil(t, arrayOfArrays.Items)
	assert.Equal(t, "array", arrayOfArrays.Items.Type)
	assert.NotNil(t, arrayOfArrays.Items.Items)
	assert.Equal(t, "string", arrayOfArrays.Items.Items.Type)

	secretString, ok := project.Config["secretString"]
	assert.True(t, ok, "should be able to read secretString")
	assert.Equal(t, "string", secretString.TypeName())
	assert.Equal(t, "", secretString.Description)
	assert.Equal(t, nil, secretString.Default)
	assert.True(t, secretString.Secret)
	assert.Nil(t, secretString.Items)
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

func TestNamespacedProjectConfigShouldNotBeExplicitlyTyped(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  aws:region:
    type: string
    value:
      region: us-west-1`

	_, projectError := loadProjectFromText(t, projectYaml)
	assert.Contains(t, projectError.Error(),
		"Configuration key 'aws:region' is not namespaced by the project and should not define a type")
}

func TestProjectConfigCannotHaveBothValueAndDefault(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  instanceSize:
    type: string
    default: t3.micro
    value: t4.large`

	_, projectError := loadProjectFromText(t, projectYaml)
	assert.Contains(t, projectError.Error(),
		"project config 'instanceSize' cannot have both a 'default' and 'value' attribute")
}

func TestProjectConfigCannotBeTypedArrayWithoutItems(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  instanceSize:
    type: array
    default: [t3.micro, t4.large]`

	_, projectError := loadProjectFromText(t, projectYaml)
	assert.Contains(t, projectError.Error(),
		"The configuration key 'instanceSize' declares an array "+
			"but does not specify the underlying type via the 'items' attribute")
}

func TestNamespacedProjectConfigShouldNotBeProvideDefault(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config:
  aws:region:
    default: us-west-1`

	_, projectError := loadProjectFromText(t, projectYaml)
	assert.Contains(t, projectError.Error(),
		"Configuration key 'aws:region' is not namespaced by the project and should not define a default value")
	assert.Contains(t, projectError.Error(),
		"Did you mean to use the 'value' attribute instead of 'default'?")
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
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t, configError.Error(), "Stack 'dev' with configuration key 'values' must be of type 'array<string>'")
}

func TestLoadingConfigIsRewrittenToStackConfigDir(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config: ./some/path`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.NoError(t, projectError, "Shold be able to load the project")
	assert.Equal(t, "./some/path", project.StackConfigDir, "Stack config dir is read from the config property")
	assert.Equal(t, 0, len(project.Config), "Config should be empty")
}

func TestDefningBothConfigAndStackConfigDirErrorsOut(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config: ./some/path
stackConfigDir: ./some/other/path`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.Nil(t, project, "Should NOT be able to load the project")
	assert.NotNil(t, projectError, "There is a project error")
	assert.Contains(t, projectError.Error(), "Should not use both config and stackConfigDir")
}

func TestConfigObjectAndStackConfigDirSuccessfullyLoadProject(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
stackConfigDir: ./some/other/path
config:
  value: hello
`

	project, projectError := loadProjectFromText(t, projectYaml)
	assert.Nil(t, projectError, "There is no error")
	assert.NotNil(t, project, "The project can be loaded correctly")
	assert.Equal(t, "./some/other/path", project.StackConfigDir)
	assert.Equal(t, 1, len(project.Config), "there is one config value")
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
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t, configError.Error(), "Stack 'dev' is missing configuration value 'values'")
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
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t, configError.Error(), "Stack 'dev' is missing configuration values 'another' and 'values'")
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
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t, configError.Error(), "Stack 'dev' is missing configuration values 'hello', 'values' and 'world'")
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

func TestProjectLoadYAML(t *testing.T) {
	t.Parallel()

	// Test wrong type
	_, err := loadProjectFromText(t, "\"hello\"")
	assert.Contains(t, err.Error(), "expected project to be an object")

	// Test bad key
	_, err = loadProjectFromText(t, "4: hello")
	assert.Contains(t, err.Error(), "expected only string keys, got '%!s(int=4)'")

	// Test nested bad key
	_, err = loadProjectFromText(t, "hello:\n    6: bad")
	assert.Contains(t, err.Error(), "project is missing a 'name' attribute")

	// Test lack of name
	_, err = loadProjectFromText(t, "{}")
	assert.Contains(t, err.Error(), "project is missing a 'name' attribute")

	// Test bad name
	_, err = loadProjectFromText(t, "name:")
	assert.Contains(t, err.Error(), "project is missing a non-empty string 'name' attribute")

	// Test missing runtime
	_, err = loadProjectFromText(t, "name: project")
	assert.Contains(t, err.Error(), "project is missing a 'runtime' attribute")

	// Test other schema errors
	_, err = loadProjectFromText(t, "name: project\nruntime: 4")
	// These can vary in order, so contains not equals check
	expected := []string{
		"3 errors occurred:",
		"* #/runtime: oneOf failed",
		"* #/runtime: expected string, but got number",
		"* #/runtime: expected object, but got number",
	}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	_, err = loadProjectFromText(t, "name: project\nruntime: test\nbackend: 4\nmain: {}")
	expected = []string{
		"2 errors occurred:",
		"* #/main: expected string or null, but got object",
		"* #/backend: expected object or null, but got number",
	}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	// Test success
	proj, err := loadProjectFromText(t, "name: project\nruntime: test")
	assert.NoError(t, err)
	assert.Equal(t, tokens.PackageName("project"), proj.Name)
	assert.Equal(t, "test", proj.Runtime.Name())

	// Test null optionals should work
	proj, err = loadProjectFromText(t, "name: project\nruntime: test\ndescription:\nmain: null\nbackend:\n")
	assert.NoError(t, err)
	assert.Nil(t, proj.Description)
	assert.Equal(t, "", proj.Main)
}

func TestProjectSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		project Project
	}{
		{
			name: "Numeric name",
			project: Project{
				Name:    "1234",
				Runtime: NewProjectRuntimeInfo("python", nil),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmp, err := os.CreateTemp("", "*.yaml")
			require.NoError(t, err)
			defer deleteFile(t, tmp)

			path := tmp.Name()

			err = tt.project.Save(path)
			require.NoError(t, err)

			loadedProject, err := LoadProject(path)
			require.NoError(t, err)
			require.NotNil(t, loadedProject)

			// Clear the raw data before we compare
			loadedProject.raw = nil
			assert.Equal(t, tt.project, *loadedProject)
		})
	}
}

func TestProjectEditRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		edit     func(*Project)
		expected string
	}{
		{
			name:     "Change name",
			yaml:     "name: test\nruntime: python\n",
			edit:     func(proj *Project) { proj.Name = "new" },
			expected: "name: new\nruntime: python\n",
		},
		{
			name: "Add runtime option",
			yaml: "name: test\nruntime: python\n",
			edit: func(proj *Project) {
				proj.Runtime = NewProjectRuntimeInfo(
					proj.Runtime.Name(),
					map[string]interface{}{
						"setting": "test",
					})
			},
			expected: "name: test\nruntime:\n  name: python\n  options:\n    setting: test\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmp, err := os.CreateTemp("", "*.yaml")
			require.NoError(t, err)
			defer deleteFile(t, tmp)

			path := tmp.Name()
			err = os.WriteFile(path, []byte(tt.yaml), 0o600)
			require.NoError(t, err)

			loadedProject, err := LoadProject(path)
			require.NoError(t, err)
			require.NotNil(t, loadedProject)

			tt.edit(loadedProject)
			err = loadedProject.Save(path)
			require.NoError(t, err)

			actualYaml, err := os.ReadFile(path)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(actualYaml))
		})
	}
}

func TestEnvironmentAppend(t *testing.T) {
	t.Parallel()

	t.Run("YAML list", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackYaml := ""

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		stack, err := loadProjectStackFromText(t, project, projectStackYaml)
		require.NoError(t, err)

		stack.Environment = stack.Environment.Append("env")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "environment:\n  - env\n", string(marshaled))

		stack.Environment = stack.Environment.Append("env2")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "environment:\n  - env\n  - env2\n", string(marshaled))
	})

	t.Run("YAML literal", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackYaml := `environment:
  values:
    pulumiConfig:
      aws:region: us-west-2`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		stack, err := loadProjectStackFromText(t, project, projectStackYaml)
		require.NoError(t, err)

		expected := `environment:
  imports:
    - env
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Append("env")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `environment:
  imports:
    - env
    - env2
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Append("env2")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))
	})
}

func TestEnvironmentRemove(t *testing.T) {
	t.Parallel()

	t.Run("YAML list", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackYaml := `environment:
  - env
  - env2
`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		stack, err := loadProjectStackFromText(t, project, projectStackYaml)
		require.NoError(t, err)

		stack.Environment = stack.Environment.Remove("env")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "environment:\n  - env2\n", string(marshaled))

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "{}\n", string(marshaled))
	})

	t.Run("YAML literal", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackYaml := `environment:
  imports:
    - env
    - env2
  values:
    pulumiConfig:
      aws:region: us-west-2`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		stack, err := loadProjectStackFromText(t, project, projectStackYaml)
		require.NoError(t, err)

		expected := `environment:
  imports:
    - env2
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Remove("env")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `environment:
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))
	})
}
