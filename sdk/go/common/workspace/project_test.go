package workspace

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
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
	invalidConfig["instanceSize"] = ProjectConfigType{
		Type:    "integer",
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
	invalidConfigWithArray := make(map[string]ProjectConfigType)
	invalidConfigWithArray["values"] = ProjectConfigType{
		Type: "array",
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
	validConfig := make(map[string]ProjectConfigType)
	validConfig["instanceSize"] = ProjectConfigType{
		Type:    "integer",
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
	validConfigWithArray := make(map[string]ProjectConfigType)
	validConfigWithArray["values"] = ProjectConfigType{
		Type: "array",
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
		tmp, err := ioutil.TempFile("", "*.json")
		assert.NoError(t, err)
		path := tmp.Name()
		err = ioutil.WriteFile(path, []byte(str), 0600)
		assert.NoError(t, err)
		return LoadProject(path)
	}

	// Test wrong type
	_, err := writeAndLoad("\"hello  \"")
	assert.Equal(t, "expected an object", err.Error())

	// Test lack of name
	_, err = writeAndLoad("{}")
	assert.Equal(t, "project is missing a 'name' attribute", err.Error())

	// Test bad name
	_, err = writeAndLoad("{\"name\": \"\"}")
	assert.Equal(t, "project is missing a non-empty string 'name' attribute", err.Error())

	// Test missing runtime
	_, err = writeAndLoad("{\"name\": \"project\"}")
	assert.Equal(t, "project is missing a 'runtime' attribute", err.Error())

	// Test other schema errors
	_, err = writeAndLoad("{\"name\": \"project\", \"runtime\": 4}")
	// These can vary in order, so contains not equals check
	expected := []string{
		"3 errors occurred:",
		"* #/runtime: oneOf failed",
		"* #/runtime: expected string, but got number",
		"* #/runtime: expected object, but got number"}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	_, err = writeAndLoad("{\"name\": \"project\", \"runtime\": \"test\", \"backend\": 4, \"main\": {}}")
	expected = []string{
		"2 errors occurred:",
		"* #/main: expected string or null, but got object",
		"* #/backend: expected object or null, but got number"}
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
	tmp, err := ioutil.TempFile("", "*.yaml")
	assert.NoError(t, err)
	path := tmp.Name()
	err = ioutil.WriteFile(path, []byte(content), 0600)
	assert.NoError(t, err)
	defer deleteFile(t, tmp)
	return LoadProject(path)
}

func loadProjectStackFromText(t *testing.T, project *Project, content string) (*ProjectStack, error) {
	tmp, err := ioutil.TempFile("", "*.yaml")
	assert.NoError(t, err)
	path := tmp.Name()
	err = ioutil.WriteFile(path, []byte(content), 0600)
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
  `

	project, err := loadProjectFromText(t, projectContent)
	assert.NoError(t, err, "Should be able to load the project")
	assert.Equal(t, 8, len(project.Config), "There are 8 config type definition")
	// full integer config schema
	integerSchemFull, ok := project.Config["integerSchemaFull"]
	assert.True(t, ok, "should be able to read integerSchemaFull")
	assert.Equal(t, "integer", integerSchemFull.Type)
	assert.Equal(t, "a very important value", integerSchemFull.Description)
	assert.Equal(t, 1, integerSchemFull.Default)
	assert.Nil(t, integerSchemFull.Items, "Primtive config type doesn't have an items type")

	integerSchemaSimple, ok := project.Config["integerSchemaSimple"]
	assert.True(t, ok, "should be able to read integerSchemaSimple")
	assert.Equal(t, "integer", integerSchemaSimple.Type, "integer type is inferred correctly")
	assert.Equal(t, 20, integerSchemaSimple.Default, "Default integer value is parsed correctly")

	textSchemaFull, ok := project.Config["textSchemaFull"]
	assert.True(t, ok, "should be able to read textSchemaFull")
	assert.Equal(t, "string", textSchemaFull.Type)
	assert.Equal(t, "t3.micro", textSchemaFull.Default)
	assert.Equal(t, "", textSchemaFull.Description)

	textSchemaSimple, ok := project.Config["textSchemaSimple"]
	assert.True(t, ok, "should be able to read textSchemaSimple")
	assert.Equal(t, "string", textSchemaSimple.Type)
	assert.Equal(t, "t4.large", textSchemaSimple.Default)

	booleanSchemaFull, ok := project.Config["booleanSchemaFull"]
	assert.True(t, ok, "should be able to read booleanSchemaFull")
	assert.Equal(t, "boolean", booleanSchemaFull.Type)
	assert.Equal(t, true, booleanSchemaFull.Default)

	booleanSchemaSimple, ok := project.Config["booleanSchemaSimple"]
	assert.True(t, ok, "should be able to read booleanSchemaSimple")
	assert.Equal(t, "boolean", booleanSchemaSimple.Type)
	assert.Equal(t, false, booleanSchemaSimple.Default)

	simpleArrayOfStrings, ok := project.Config["simpleArrayOfStrings"]
	assert.True(t, ok, "should be able to read simpleArrayOfStrings")
	assert.Equal(t, "array", simpleArrayOfStrings.Type)
	assert.NotNil(t, simpleArrayOfStrings.Items)
	assert.Equal(t, "string", simpleArrayOfStrings.Items.Type)
	arrayValues := simpleArrayOfStrings.Default.([]interface{})
	assert.Equal(t, "hello", arrayValues[0])

	arrayOfArrays, ok := project.Config["arrayOfArrays"]
	assert.True(t, ok, "should be able to read arrayOfArrays")
	assert.Equal(t, "array", arrayOfArrays.Type)
	assert.NotNil(t, arrayOfArrays.Items)
	assert.Equal(t, "array", arrayOfArrays.Items.Type)
	assert.NotNil(t, arrayOfArrays.Items.Items)
	assert.Equal(t, "string", arrayOfArrays.Items.Items.Type)
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
	configError := ValidateStackConfigAndApplyProjectConfig("dev", project, stack.Config)
	assert.NoError(t, configError, "Config override should be valid")

	assert.Equal(t, 3, len(stack.Config), "Stack config now has three values")
	// value of instanceSize is overwritten from the stack
	assert.Equal(t, "t4.large", getConfigValue(t, stack.Config, "test:instanceSize"))
	// instanceCount and protect are inherited from the project
	assert.Equal(t, "20", getConfigValue(t, stack.Config, "test:instanceCount"))
	assert.Equal(t, "true", getConfigValue(t, stack.Config, "test:protect"))
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
	configError := ValidateStackConfigAndApplyProjectConfig("dev", project, stack.Config)
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t, configError.Error(), "Stack 'dev' with configuration key 'values' must be of type 'array<string>'")
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
	configError := ValidateStackConfigAndApplyProjectConfig("dev", project, stack.Config)
	assert.NoError(t, configError, "there should no config type error")

	invalidStackConfig, stackError := loadProjectStackFromText(t, project, projectStackYamlInvalid)
	assert.NoError(t, stackError, "Should be able to read the stack")
	configError = ValidateStackConfigAndApplyProjectConfig("dev", project, invalidStackConfig.Config)
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
	configError := ValidateStackConfigAndApplyProjectConfig("dev", project, stack.Config)
	assert.NotNil(t, configError, "there should be a config type error")
	assert.Contains(t, configError.Error(), "Stack 'dev' missing configuration value 'values'")
}

func TestProjectLoadYAML(t *testing.T) {
	t.Parallel()

	// Test wrong type
	_, err := loadProjectFromText(t, "\"hello\"")
	assert.Equal(t, "expected an object", err.Error())

	// Test bad key
	_, err = loadProjectFromText(t, "4: hello")
	assert.Equal(t, "expected only string keys, got '%!s(int=4)'", err.Error())

	// Test nested bad key
	_, err = loadProjectFromText(t, "hello:\n    6: bad")
	assert.Equal(t, "expected only string keys, got '%!s(int=6)'", err.Error())

	// Test lack of name
	_, err = loadProjectFromText(t, "{}")
	assert.Equal(t, "project is missing a 'name' attribute", err.Error())

	// Test bad name
	_, err = loadProjectFromText(t, "name:")
	assert.Equal(t, "project is missing a non-empty string 'name' attribute", err.Error())

	// Test missing runtime
	_, err = loadProjectFromText(t, "name: project")
	assert.Equal(t, "project is missing a 'runtime' attribute", err.Error())

	// Test other schema errors
	_, err = loadProjectFromText(t, "name: project\nruntime: 4")
	// These can vary in order, so contains not equals check
	expected := []string{
		"3 errors occurred:",
		"* #/runtime: oneOf failed",
		"* #/runtime: expected string, but got number",
		"* #/runtime: expected object, but got number"}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	_, err = loadProjectFromText(t, "name: project\nruntime: test\nbackend: 4\nmain: {}")
	expected = []string{
		"2 errors occurred:",
		"* #/main: expected string or null, but got object",
		"* #/backend: expected object or null, but got number"}
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
