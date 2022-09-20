package workspace

import (
	"encoding/json"
	"io/ioutil"
	"testing"

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

func TestProjectLoadYAML(t *testing.T) {
	t.Parallel()

	writeAndLoad := func(str string) (*Project, error) {
		tmp, err := ioutil.TempFile("", "*.yaml")
		assert.NoError(t, err)
		path := tmp.Name()
		err = ioutil.WriteFile(path, []byte(str), 0600)
		assert.NoError(t, err)
		return LoadProject(path)
	}

	// Test wrong type
	_, err := writeAndLoad("\"hello\"")
	assert.Equal(t, "expected an object", err.Error())

	// Test bad key
	_, err = writeAndLoad("4: hello")
	assert.Equal(t, "expected only string keys, got '%!s(int=4)'", err.Error())

	// Test nested bad key
	_, err = writeAndLoad("hello:\n    6: bad")
	assert.Equal(t, "expected only string keys, got '%!s(int=6)'", err.Error())

	// Test lack of name
	_, err = writeAndLoad("{}")
	assert.Equal(t, "project is missing a 'name' attribute", err.Error())

	// Test bad name
	_, err = writeAndLoad("name:")
	assert.Equal(t, "project is missing a non-empty string 'name' attribute", err.Error())

	// Test missing runtime
	_, err = writeAndLoad("name: project")
	assert.Equal(t, "project is missing a 'runtime' attribute", err.Error())

	// Test other schema errors
	_, err = writeAndLoad("name: project\nruntime: 4")
	// These can vary in order, so contains not equals check
	expected := []string{
		"3 errors occurred:",
		"* #/runtime: oneOf failed",
		"* #/runtime: expected string, but got number",
		"* #/runtime: expected object, but got number"}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	_, err = writeAndLoad("name: project\nruntime: test\nbackend: 4\nmain: {}")
	expected = []string{
		"2 errors occurred:",
		"* #/main: expected string or null, but got object",
		"* #/backend: expected object or null, but got number"}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	// Test success
	proj, err := writeAndLoad("name: project\nruntime: test")
	assert.NoError(t, err)
	assert.Equal(t, tokens.PackageName("project"), proj.Name)
	assert.Equal(t, "test", proj.Runtime.Name())

	// Test null optionals should work
	proj, err = writeAndLoad("name: project\nruntime: test\ndescription:\nmain: null\nbackend:\n")
	assert.NoError(t, err)
	assert.Nil(t, proj.Description)
	assert.Equal(t, "", proj.Main)
}
