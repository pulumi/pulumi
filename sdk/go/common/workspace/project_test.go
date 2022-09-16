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

func TestProjectValidation(t *testing.T) {
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
		"* #/main: expected string, but got object",
		"* #/backend: expected object, but got number"}
	for _, e := range expected {
		assert.Contains(t, err.Error(), e)
	}

	// Test success
	proj, err := writeAndLoad("{\"name\": \"project\", \"runtime\": \"test\"}")
	assert.NoError(t, err)
	assert.Equal(t, tokens.PackageName("project"), proj.Name)
	assert.Equal(t, "test", proj.Runtime.Name())
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
