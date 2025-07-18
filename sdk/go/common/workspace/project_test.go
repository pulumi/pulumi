// Copyright 2018-2024, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
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
		require.NoError(t, err)

		var riRountrip ProjectRuntimeInfo
		err = unmarshal(byts, &riRountrip)
		require.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.Nil(t, riRountrip.Options())

		ri = NewProjectRuntimeInfo("nodejs", map[string]interface{}{
			"typescript":   true,
			"stringOption": "hello",
		})
		byts, err = marshal(ri)
		require.NoError(t, err)
		err = unmarshal(byts, &riRountrip)
		require.NoError(t, err)
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
	assert.EqualError(t, err, "project is missing a 'name' attribute")
	// Test lack of runtime
	proj.Name = "a project"
	err = proj.Validate()
	assert.EqualError(t, err, "project is missing a 'runtime' attribute")

	// Test success
	proj.Runtime = NewProjectRuntimeInfo("test", nil)
	err = proj.Validate()
	require.NoError(t, err)
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
	assert.ErrorContains(t, err,
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
	assert.ErrorContains(t, err,
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
	require.NoError(t, err, "There should be no validation error")

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
	require.NoError(t, err, "There should be no validation error")
}

func writeAndLoad(t *testing.T, str string) (*Project, error) {
	tmp, err := os.CreateTemp(t.TempDir(), "*.json")
	require.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(str), 0o600)
	require.NoError(t, err)
	return LoadProject(path)
}

func TestProjectLoadJSON(t *testing.T) {
	t.Parallel()

	// Test wrong type
	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, "\"hello  \"")

		// Assert.
		assert.ErrorContains(t, err, "expected project to be an object, was 'string'")
	})

	t.Run("missing name attribute", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, "{}")

		// Assert.
		assert.ErrorContains(t, err, "project is missing a 'name' attribute")
	})

	t.Run("bad name", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, "{\"name\": \"\"}")

		// Assert.
		assert.ErrorContains(t, err, "project is missing a non-empty string 'name' attribute")
	})

	t.Run("missing runtime", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, "{\"name\": \"project\"}")

		// Assert.
		assert.ErrorContains(t, err, "project is missing a 'runtime' attribute")
	})

	t.Run("multiple errors 1", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, "{\"name\": \"project\", \"runtime\": 4}")

		// Assert.
		// The order can vary here, so we use Contains and not Equals.
		expected := []string{
			"3 errors occurred:",
			"* #/runtime: oneOf failed",
			"* #/runtime: expected string, but got number",
			"* #/runtime: expected object, but got number",
		}

		for _, e := range expected {
			assert.ErrorContains(t, err, e)
		}
	})

	t.Run("multiple errors, 2", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, "{\"name\": \"project\", \"runtime\": \"test\", \"backend\": 4, \"main\": {}}")

		// Assert.
		// The order can vary here, so we use Contains and not Equals.
		expected := []string{
			"2 errors occurred:",
			"* #/main: expected string or null, but got object",
			"* #/backend: expected object or null, but got number",
		}

		for _, e := range expected {
			assert.ErrorContains(t, err, e)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// Act.
		proj, err := writeAndLoad(t, "{\"name\": \"project\", \"runtime\": \"test\"}")

		// Assert.
		require.NoError(t, err)
		assert.Equal(t, tokens.PackageName("project"), proj.Name)
		assert.Equal(t, "test", proj.Runtime.Name())
	})

	t.Run("null optionals should work", func(t *testing.T) {
		t.Parallel()

		// Act.
		proj, err := writeAndLoad(t, "{\"name\": \"project\", \"runtime\": \"test\", "+
			"\"description\": null, \"main\": null, \"backend\": null}")

		// Assert.
		require.NoError(t, err)
		assert.Nil(t, proj.Description)
		assert.Equal(t, "", proj.Main)
	})
}

func TestProjectLoadJSONInformativeErrors(t *testing.T) {
	t.Parallel()

	t.Run("a missing name attribute", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{"Name": "project", "runtime": "test"}`)

		// Assert.
		assert.ErrorContains(t, err, "project is missing a 'name' attribute")
		assert.ErrorContains(t, err, "found 'Name' instead")
	})

	t.Run("a missing runtime attribute", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{"name": "project", "rutnime": "test"}`)

		// Assert.
		assert.ErrorContains(t, err, "project is missing a 'runtime' attribute")
		assert.ErrorContains(t, err, "found 'rutnime' instead")
	})

	t.Run("a minor spelling mistake in a schema field", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{
  "name": "project",
  "runtime": "test",
  "template": {
    "displatName": "foo"
  }
}`)

		// Assert.
		assert.ErrorContains(t, err, "'displatName' not allowed; did you mean 'displayName'?")
	})

	t.Run("a major spelling mistake in a schema field", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{
  "name": "project",
  "runtime": "test",
  "template": {
    "displayNameDisplayName": "foo"
  }
}`)

		// Assert.
		assert.ErrorContains(t, err, "'displayNameDisplayName' not allowed")
		assert.ErrorContains(t, err, "'displayNameDisplayName' not allowed; the allowed attributes are "+
			"'config', 'description', 'displayName', 'important', 'metadata' and 'quickstart'")
	})

	t.Run("specific errors when only a single attribute is expected", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{
  "name": "project",
  "runtime": "test",
  "backend": {
    "url": "https://pulumi.com",
    "name": "test"
  }
}`)

		// Assert.
		assert.ErrorContains(t, err, "'name' not allowed")
		assert.ErrorContains(t, err, "'name' not allowed; the only allowed attribute is 'url'")
	})

	t.Run("a minor spelling mistake even deeper in the schema", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{
  "name": "project",
  "runtime": "test",
  "plugins": {
    "providers": [
      {
        "nome": "test"
      }
    ]
  }
}`)

		// Assert.
		assert.ErrorContains(t, err, "'nome' not allowed; did you mean 'name'")
	})

	t.Run("a major spelling mistake even deeper in the schema", func(t *testing.T) {
		t.Parallel()

		// Act.
		_, err := writeAndLoad(t, `{
  "name": "project",
  "runtime": "test",
  "plugins": {
    "providers": [
      {
        "displayName": "test"
      }
    ]
  }
}`)

		// Assert.
		assert.ErrorContains(t, err, "'displayName' not allowed")
		assert.ErrorContains(t, err, "'displayName' not allowed; the allowed attributes are "+
			"'name', 'path' and 'version'")
	})
}

func deleteFile(t *testing.T, file *os.File) {
	if file != nil {
		err := os.Remove(file.Name())
		require.NoError(t, err, "Error while deleting file")
	}
}

func loadProjectFromText(t *testing.T, content string) (*Project, error) {
	tmp, err := os.CreateTemp(t.TempDir(), "*.yaml")
	require.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	defer deleteFile(t, tmp)
	return LoadProject(path)
}

func loadProjectStackFromText(t *testing.T, sink diag.Sink, project *Project, content string) (*ProjectStack, error) {
	tmp, err := os.CreateTemp(t.TempDir(), "*.yaml")
	require.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	defer deleteFile(t, tmp)
	return LoadProjectStack(sink, project, path)
}

func loadProjectStackFromJSONText(
	t *testing.T,
	sink diag.Sink,
	project *Project,
	content string,
) (*ProjectStack, error) {
	tmp, err := os.CreateTemp(t.TempDir(), "*.json")
	require.NoError(t, err)
	path := tmp.Name()
	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	defer deleteFile(t, tmp)
	return LoadProjectStack(sink, project, path)
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
	require.NoError(t, err, "Should be able to load the project")
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
	require.NotNil(t, simpleArrayOfStrings.Items)
	assert.Equal(t, "string", simpleArrayOfStrings.Items.Type)
	arrayValues := simpleArrayOfStrings.Default.([]interface{})
	assert.Equal(t, "hello", arrayValues[0])

	arrayOfArrays, ok := project.Config["arrayOfArrays"]
	assert.True(t, ok, "should be able to read arrayOfArrays")
	assert.Equal(t, "array", arrayOfArrays.TypeName())
	assert.False(t, arrayOfArrays.Secret)
	require.NotNil(t, arrayOfArrays.Items)
	assert.Equal(t, "array", arrayOfArrays.Items.Type)
	require.NotNil(t, arrayOfArrays.Items.Items)
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
	require.NoErrorf(t, err, "There should be no error parsing the config key '%v'", key)
	configValue, foundValue := stackConfig[parsedKey]
	assert.Truef(t, foundValue, "Couldn't find a value for config key %v", key)
	value, valueError := configValue.Value(config.NopDecrypter)
	require.NoErrorf(t, valueError, "Error while getting the value for key %v", key)
	return value
}

func getConfigValueUnmarshalled(t *testing.T, stackConfig config.Map, key string) interface{} {
	parsedKey, err := config.ParseKey(key)
	require.NoErrorf(t, err, "There should be no error parsing the config key '%v'", key)
	configValue, foundValue := stackConfig[parsedKey]
	assert.Truef(t, foundValue, "Couldn't find a value for config key %v", key)
	valueJSON, valueError := configValue.Value(config.NopDecrypter)
	require.NoErrorf(t, valueError, "Error while getting the value for key %v", key)
	var value interface{}
	err = json.Unmarshal([]byte(valueJSON), &value)
	require.NoErrorf(t, err, "Error while unmarshalling value for key %v", key)
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")

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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")

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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")
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
	assert.ErrorContains(t, projectError,
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
	assert.ErrorContains(t, projectError,
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
	assert.ErrorContains(t, projectError,
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
	assert.ErrorContains(t, projectError,
		"Configuration key 'aws:region' is not namespaced by the project and should not define a default value")
	assert.ErrorContains(t, projectError,
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.ErrorContains(t, configError, "Stack 'dev' with configuration key 'values' must be of type 'array<string>'")
}

func TestLoadingConfigIsRewrittenToStackConfigDir(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet
config: ./some/path`

	project, projectError := loadProjectFromText(t, projectYaml)
	require.NoError(t, projectError, "Shold be able to load the project")
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
	assert.ErrorContains(t, projectError, "Should not use both config and stackConfigDir")
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
	require.NotNil(t, project, "The project can be loaded correctly")
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

	ctx := context.Background()
	project, projectError := loadProjectFromText(t, projectYaml)
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYamlValid)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		ctx,
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "there should no config type error")

	invalidStackConfig, stackError := loadProjectStackFromText(t, sink, project, projectStackYamlInvalid)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError = ValidateStackConfigAndApplyProjectConfig(
		ctx,
		"dev",
		project,
		esc.Value{},
		invalidStackConfig.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	assert.ErrorContains(t, configError,
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
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
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
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
	require.NoError(t, err)

	projectStackYamlValid := fmt.Sprintf(`
config:
  test:importantNumber:
    secure: %s
`, encryptedValue)

	projectStackYamlInvalid := `
config:
  test:importantNumber: 20
`

	ctx := context.Background()
	project, projectError := loadProjectFromText(t, projectYaml)
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYamlValid)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		ctx,
		"dev",
		project,
		esc.Value{},
		stack.Config,
		crypter,
		crypter)
	require.NoError(t, configError, "there should no config type error")

	invalidStackConfig, stackError := loadProjectStackFromText(t, sink, project, projectStackYamlInvalid)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError = ValidateStackConfigAndApplyProjectConfig(
		ctx,
		"dev",
		project,
		esc.Value{},
		invalidStackConfig.Config,
		crypter,
		crypter)
	assert.ErrorContains(t, configError,
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
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, stackYAML)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")

	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
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
	assert.ErrorContains(t, err, "expected project to be an object")

	// Test bad key
	_, err = loadProjectFromText(t, "4: hello")
	assert.ErrorContains(t, err, "expected only string keys, got '%!s(int=4)'")

	// Test nested bad key
	_, err = loadProjectFromText(t, "hello:\n    6: bad")
	assert.ErrorContains(t, err, "project is missing a 'name' attribute")

	// Test lack of name
	_, err = loadProjectFromText(t, "{}")
	assert.ErrorContains(t, err, "project is missing a 'name' attribute")

	// Test bad name
	_, err = loadProjectFromText(t, "name:")
	assert.ErrorContains(t, err, "project is missing a non-empty string 'name' attribute")

	// Test missing runtime
	_, err = loadProjectFromText(t, "name: project")
	assert.ErrorContains(t, err, "project is missing a 'runtime' attribute")

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
		assert.ErrorContains(t, err, e)
	}

	_, err = loadProjectFromText(t, "name: project\nruntime: test\nbackend: 4\nmain: {}")
	expected = []string{
		"2 errors occurred:",
		"* #/main: expected string or null, but got object",
		"* #/backend: expected object or null, but got number",
	}
	for _, e := range expected {
		assert.ErrorContains(t, err, e)
	}

	// Test success
	proj, err := loadProjectFromText(t, "name: project\nruntime: test")
	require.NoError(t, err)
	assert.Equal(t, tokens.PackageName("project"), proj.Name)
	assert.Equal(t, "test", proj.Runtime.Name())

	// Test null optionals should work
	proj, err = loadProjectFromText(t, "name: project\nruntime: test\ndescription:\nmain: null\nbackend:\n")
	require.NoError(t, err)
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

			tmp, err := os.CreateTemp(t.TempDir(), "*.yaml")
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

			tmp, err := os.CreateTemp(t.TempDir(), "*.yaml")
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

	t.Run("JSON list", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackJSON := "{}"

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromJSONText(t, sink, project, projectStackJSON)
		require.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		stack.Environment = stack.Environment.Append("env")
		marshaled, err := encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "{\n    \"environment\": [\n        \"env\"\n    ]\n}\n", string(marshaled))

		stack.Environment = stack.Environment.Append("env2")
		marshaled, err = encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "{\n    \"environment\": [\n        \"env\",\n        \"env2\"\n    ]\n}\n", string(marshaled))
	})

	t.Run("JSON literal", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackJSON := `{
    "environment": {
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromJSONText(t, sink, project, projectStackJSON)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		require.NoError(t, err)

		expected := `{
    "environment": {
        "imports": [
            "env"
        ],
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		stack.Environment = stack.Environment.Append("env")
		marshaled, err := encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `{
    "environment": {
        "imports": [
            "env",
            "env2"
        ],
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		stack.Environment = stack.Environment.Append("env2")
		marshaled, err = encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))
	})

	t.Run("YAML list", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackYaml := ""

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromText(t, sink, project, projectStackYaml)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
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
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromText(t, sink, project, projectStackYaml)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		require.NoError(t, err)

		expected := `environment:
  values:
    pulumiConfig:
      aws:region: us-west-2
  imports:
    - env
`

		stack.Environment = stack.Environment.Append("env")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `environment:
  values:
    pulumiConfig:
      aws:region: us-west-2
  imports:
    - env
    - env2
`

		stack.Environment = stack.Environment.Append("env2")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))
	})
}

func TestEnvironmentRemove(t *testing.T) {
	t.Parallel()

	t.Run("JSON list", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackJSON := `{
    "environment": [
        "env2",
        "env",
        "env2"
    ]
}
`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromJSONText(t, sink, project, projectStackJSON)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		require.NoError(t, err)

		expected := `{
    "environment": [
        "env2",
        "env"
    ]
}
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err := encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `{
    "environment": [
        "env2"
    ]
}
`

		stack.Environment = stack.Environment.Remove("env")
		marshaled, err = encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err = encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, "{}\n", string(marshaled))
	})

	t.Run("JSON literal", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackJSON := `{
    "environment": {
        "imports": [
            {
                "env2": {
                    "merge": false
                }
            },
            "env",
            "env2"
        ],
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromJSONText(t, sink, project, projectStackJSON)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		require.NoError(t, err)

		expected := `{
    "environment": {
        "imports": [
            {
                "env2": {
                    "merge": false
                }
            },
            "env"
        ],
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err := encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `{
    "environment": {
        "imports": [
            "env"
        ],
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err = encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `{
    "environment": {
        "imports": [],
        "values": {
            "pulumiConfig": {
                "aws:region": "us-west-2"
            }
        }
    }
}
`

		stack.Environment = stack.Environment.Remove("env")
		marshaled, err = encoding.JSON.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))
	})

	t.Run("YAML list", func(t *testing.T) {
		t.Parallel()

		projectYaml := `name: test
runtime: yaml`

		projectStackYaml := `environment:
  - env2
  - env
  - env2
`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromText(t, sink, project, projectStackYaml)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		require.NoError(t, err)

		expected := `environment:
  - env2
  - env
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		stack.Environment = stack.Environment.Remove("env")
		marshaled, err = encoding.YAML.Marshal(stack)
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
    - {env2: {merge: false}}
    - env
    - env2
  values:
    pulumiConfig:
      aws:region: us-west-2`

		project, err := loadProjectFromText(t, projectYaml)
		require.NoError(t, err)
		var stdout, stderr bytes.Buffer
		sink := diagtest.MockSink(&stdout, &stderr)
		stack, err := loadProjectStackFromText(t, sink, project, projectStackYaml)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		require.NoError(t, err)

		expected := `environment:
  imports:
    - {env2: {merge: false}}
    - env
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err := encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `environment:
  imports:
    - env
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Remove("env2")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))

		expected = `environment:
  values:
    pulumiConfig:
      aws:region: us-west-2
`

		stack.Environment = stack.Environment.Remove("env")
		marshaled, err = encoding.YAML.Marshal(stack)
		require.NoError(t, err)
		assert.Equal(t, expected, string(marshaled))
	})
}

// Regression test for https://github.com/pulumi/pulumi/issues/18581, check that we handle uint64's in yaml
func TestStackConfigUInt64(t *testing.T) {
	t.Parallel()
	projectYaml := `
name: test
runtime: dotnet`

	projectStackYaml := `
config:
  test:instanceSize: 18446744073709551615`

	project, projectError := loadProjectFromText(t, projectYaml)
	require.NoError(t, projectError, "Shold be able to load the project")
	var stdout, stderr bytes.Buffer
	sink := diagtest.MockSink(&stdout, &stderr)
	stack, stackError := loadProjectStackFromText(t, sink, project, projectStackYaml)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	require.NoError(t, stackError, "Should be able to read the stack")
	configError := ValidateStackConfigAndApplyProjectConfig(
		context.Background(),
		"dev",
		project,
		esc.Value{},
		stack.Config,
		config.NewPanicCrypter(),
		config.NewPanicCrypter())
	require.NoError(t, configError, "Config override should be valid")

	assert.Equal(t, 1, len(stack.Config), "Stack config now has three values")
	assert.Equal(t, "18446744073709551615", getConfigValue(t, stack.Config, "test:instanceSize"))
}

func TestPackageValueSerialization(t *testing.T) {
	t.Parallel()

	t.Run("JSON", func(t *testing.T) {
		t.Parallel()

		// Test both simple string and complex object packages in the same Project
		proj := &Project{
			Name:    "test-project",
			Runtime: NewProjectRuntimeInfo("nodejs", nil),
			Packages: map[string]packageValue{
				"simple": {value: "github.com/example/simple-package"},
				"complex": {value: PackageSpec{
					Source:     "github.com/example/complex-package",
					Version:    "1.0.0",
					Parameters: []string{"arg1", "arg2"},
				}},
			},
		}

		// Serialize to JSON
		bytes, err := json.Marshal(proj)
		require.NoError(t, err)

		// Verify JSON contains the expected package formats
		jsonStr := string(bytes)
		assert.Contains(t, jsonStr, `"packages":`)
		assert.Contains(t, jsonStr, `"simple":"github.com/example/simple-package"`)
		assert.Contains(t, jsonStr,
			`"complex":{"source":"github.com/example/complex-package","version":"1.0.0","parameters":["arg1","arg2"]}`)

		// Deserialize back
		var newProj Project
		err = json.Unmarshal(bytes, &newProj)
		require.NoError(t, err)

		// Verify packages were correctly deserialized
		specs := newProj.GetPackageSpecs()
		assert.Equal(t, 2, len(specs))

		assert.Equal(t, "github.com/example/simple-package", specs["simple"].Source)
		assert.Empty(t, specs["simple"].Version)
		assert.Empty(t, specs["simple"].Parameters)

		assert.Equal(t, "github.com/example/complex-package", specs["complex"].Source)
		assert.Equal(t, "1.0.0", specs["complex"].Version)
		assert.Equal(t, []string{"arg1", "arg2"}, specs["complex"].Parameters)
	})

	t.Run("YAML", func(t *testing.T) {
		t.Parallel()

		// Test both simple string and complex object packages in the same Project
		proj := &Project{
			Name:    "test-project",
			Runtime: NewProjectRuntimeInfo("nodejs", nil),
			Packages: map[string]packageValue{
				"simple": {value: "github.com/example/simple-package"},
				"complex": {value: PackageSpec{
					Source:     "github.com/example/complex-package",
					Version:    "1.0.0",
					Parameters: []string{"arg1", "arg2"},
				}},
			},
		}

		// Serialize to YAML
		bytes, err := yaml.Marshal(proj)
		require.NoError(t, err)

		// Verify YAML contains the expected package formats
		yamlStr := string(bytes)
		assert.Contains(t, yamlStr, "packages:")
		assert.Contains(t, yamlStr, "simple: github.com/example/simple-package")
		assert.Contains(t, yamlStr, "complex:")
		assert.Contains(t, yamlStr, "source: github.com/example/complex-package")
		assert.Contains(t, yamlStr, "version: 1.0.0")
		assert.Contains(t, yamlStr, "parameters:")
		assert.Contains(t, yamlStr, "- arg1")
		assert.Contains(t, yamlStr, "- arg2")

		// Deserialize back
		var newProj Project
		err = yaml.Unmarshal(bytes, &newProj)
		require.NoError(t, err)

		// Verify packages were correctly deserialized
		specs := newProj.GetPackageSpecs()
		assert.Equal(t, 2, len(specs))

		assert.Equal(t, "github.com/example/simple-package", specs["simple"].Source)
		assert.Empty(t, specs["simple"].Version)
		assert.Empty(t, specs["simple"].Parameters)

		assert.Equal(t, "github.com/example/complex-package", specs["complex"].Source)
		assert.Equal(t, "1.0.0", specs["complex"].Version)
		assert.Equal(t, []string{"arg1", "arg2"}, specs["complex"].Parameters)
	})

	t.Run("Deserialization Edge Cases", func(t *testing.T) {
		t.Parallel()

		// Test invalid package value (should fail)
		jsonData := `{"name":"test-project","runtime":"nodejs","packages":{"invalid":123}}`
		var proj Project
		err := json.Unmarshal([]byte(jsonData), &proj)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "package must be either a string or a package specification object")
	})
}

func TestGetPackageSpecs(t *testing.T) {
	t.Parallel()

	// Test with nil packages
	proj := &Project{
		Name:     "test-project",
		Runtime:  NewProjectRuntimeInfo("nodejs", nil),
		Packages: nil,
	}
	specs := proj.GetPackageSpecs()
	assert.Nil(t, specs)

	// Test with empty packages
	proj = &Project{
		Name:     "test-project",
		Runtime:  NewProjectRuntimeInfo("nodejs", nil),
		Packages: map[string]packageValue{},
	}
	specs = proj.GetPackageSpecs()
	assert.Empty(t, specs)

	// Test with mixed packages
	proj = &Project{
		Name:    "test-project",
		Runtime: NewProjectRuntimeInfo("nodejs", nil),
		Packages: map[string]packageValue{
			"str": {value: "github.com/example/string-package@0.1.2"},
			"obj": {value: PackageSpec{
				Source:     "github.com/example/object-package",
				Version:    "1.2.3",
				Parameters: []string{"--arg1", "--arg2"},
			}},
		},
	}
	specs = proj.GetPackageSpecs()
	assert.Equal(t, 2, len(specs))

	assert.Equal(t, "github.com/example/string-package", specs["str"].Source)
	assert.Equal(t, "0.1.2", specs["str"].Version)
	assert.Empty(t, specs["str"].Parameters)

	assert.Equal(t, "github.com/example/object-package", specs["obj"].Source)
	assert.Equal(t, "1.2.3", specs["obj"].Version)
	assert.Equal(t, []string{"--arg1", "--arg2"}, specs["obj"].Parameters)
}

func TestAddPackage(t *testing.T) {
	t.Parallel()

	// Test adding the first package with only source
	t.Run("AddFirstPackage", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
		}

		proj.AddPackage("simple-package", PackageSpec{
			Source: "github.com/org/simple-package",
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "simple-package")
		require.Equal(t, "github.com/org/simple-package", specs["simple-package"].Source)
		require.Empty(t, specs["simple-package"].Version)
		require.Empty(t, specs["simple-package"].Parameters)

		// The internal representation should be a string for the new package
		_, ok := proj.Packages["simple-package"].value.(string)
		require.True(t, ok, "Simple package should be stored as string")
	})

	// Test adding a package with source and version
	t.Run("AddFirstPackageWithVersion", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
		}

		proj.AddPackage("versioned-package", PackageSpec{
			Source:  "github.com/org/versioned-package",
			Version: "v1.2.3",
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "versioned-package")
		require.Equal(t, "github.com/org/versioned-package", specs["versioned-package"].Source)
		require.Equal(t, "v1.2.3", specs["versioned-package"].Version)
		require.Empty(t, specs["versioned-package"].Parameters)

		// The internal representation should be a string for the new package
		_, ok := proj.Packages["versioned-package"].value.(string)
		require.True(t, ok, "Simple package should be stored as string")
	})

	// Test adding a package with parameters (should use PackageSpec format)
	t.Run("AddPackageWithParameters", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
		}

		proj.AddPackage("param-package", PackageSpec{
			Source:     "github.com/org/param-package",
			Parameters: []string{"param1", "param2"},
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "param-package")
		require.Equal(t, "github.com/org/param-package", specs["param-package"].Source)
		require.Empty(t, specs["param-package"].Version)
		require.Equal(t, []string{"param1", "param2"}, specs["param-package"].Parameters)

		// The internal representation should be a PackageSpec for the new package
		_, ok := proj.Packages["param-package"].value.(PackageSpec)
		require.True(t, ok, "Parameterized package should be stored as PackageSpec")
	})

	// Test maintaining the format consistency when adding multiple packages
	t.Run("MaintainFormatConsistency", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
			Packages: map[string]packageValue{
				"existing-package": {value: PackageSpec{
					Source:  "github.com/org/existing-package",
					Version: "v1.0.0",
				}},
			},
		}

		// Adding a new package should use the same format as existing packages
		proj.AddPackage("new-package", PackageSpec{
			Source: "github.com/org/new-package",
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "existing-package")
		require.Contains(t, specs, "new-package")
		require.Equal(t, "github.com/org/existing-package", specs["existing-package"].Source)
		require.Equal(t, "v1.0.0", specs["existing-package"].Version)

		// The internal representation should be a PackageSpec for the new package
		val, ok := proj.Packages["new-package"].value.(PackageSpec)
		require.True(t, ok, "Parameterized package should be stored as PackageSpec")
		require.Equal(t, "github.com/org/new-package", val.Source)
		require.Empty(t, val.Version)
		require.Empty(t, val.Parameters)
	})

	// Test replacing an existing package
	t.Run("ReplaceExistingPackage", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
			Packages: map[string]packageValue{
				"existing-package": {value: "github.com/org/existing-package@v1.0.0"},
			},
		}

		// Replace the existing package with a new version
		proj.AddPackage("existing-package", PackageSpec{
			Source:  "github.com/org/existing-package",
			Version: "v2.0.0",
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "existing-package")
		require.Equal(t, "github.com/org/existing-package", specs["existing-package"].Source)
		require.Equal(t, "v2.0.0", specs["existing-package"].Version)

		// The internal representation should be a string for the new package
		_, ok := proj.Packages["existing-package"].value.(string)
		require.True(t, ok, "Existing package should be stored as string")
	})

	// One existing package as string, one as spec - new one should be added as string
	t.Run("MixedFormatDefaultsToString", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
			Packages: map[string]packageValue{
				"string-package": {value: "github.com/org/string-package@v1.0.0"},
				"spec-package": {value: PackageSpec{
					Source:     "github.com/org/spec-package",
					Version:    "v1.0.0",
					Parameters: []string{"--param"},
				}},
			},
		}

		// Adding a new simple package should default to string format when there's a mix
		proj.AddPackage("new-package", PackageSpec{
			Source:  "github.com/org/new-package",
			Version: "v1.2.3",
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "new-package")
		require.Equal(t, "github.com/org/new-package", specs["new-package"].Source)
		require.Equal(t, "v1.2.3", specs["new-package"].Version)

		// The internal representation should be a string for the new package
		_, ok := proj.Packages["new-package"].value.(string)
		require.True(t, ok, "With mixed formats, new simple package should be stored as string")
	})

	// Existing package format is preserved when replacing
	t.Run("PreserveFormatWhenReplacing", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
			Packages: map[string]packageValue{
				"string-package": {value: "github.com/org/string-package@v1.0.0"},
				"spec-package": {value: PackageSpec{
					Source:  "github.com/org/spec-package",
					Version: "v1.0.0",
				}},
			},
		}

		// Replace the string package with a new version (should stay as string)
		proj.AddPackage("string-package", PackageSpec{
			Source:  "github.com/org/string-package",
			Version: "v2.0.0",
		})

		// Replace the spec package with a new version (should stay as spec)
		proj.AddPackage("spec-package", PackageSpec{
			Source:  "github.com/org/spec-package",
			Version: "v2.0.0",
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "string-package")
		require.Contains(t, specs, "spec-package")
		require.Equal(t, "github.com/org/string-package", specs["string-package"].Source)
		require.Equal(t, "v2.0.0", specs["string-package"].Version)
		require.Equal(t, "github.com/org/spec-package", specs["spec-package"].Source)
		require.Equal(t, "v2.0.0", specs["spec-package"].Version)

		// Verify the internal representation is maintained
		_, isString := proj.Packages["string-package"].value.(string)
		require.True(t, isString, "String package format should be preserved")

		_, isSpec := proj.Packages["spec-package"].value.(PackageSpec)
		require.True(t, isSpec, "PackageSpec format should be preserved")
	})

	// Format conversion when parameters added to existing string package
	t.Run("FormatConversionWhenParametersAdded", func(t *testing.T) {
		t.Parallel()

		proj := &Project{
			Name: "test-project",
			Runtime: ProjectRuntimeInfo{
				name: "nodejs",
			},
			Packages: map[string]packageValue{
				"string-package": {value: "github.com/org/string-package@v1.0.0"},
			},
		}

		// Add parameters to an existing string package - should convert to PackageSpec
		proj.AddPackage("string-package", PackageSpec{
			Source:     "github.com/org/string-package",
			Version:    "v1.0.0",
			Parameters: []string{"--new-param"},
		})

		specs := proj.GetPackageSpecs()
		require.Contains(t, specs, "string-package")
		require.Equal(t, "github.com/org/string-package", specs["string-package"].Source)
		require.Equal(t, "v1.0.0", specs["string-package"].Version)
		require.Equal(t, []string{"--new-param"}, specs["string-package"].Parameters)

		// Verify the internal representation was converted to PackageSpec
		_, isSpec := proj.Packages["string-package"].value.(PackageSpec)
		require.True(t, isSpec, "Package should be converted to PackageSpec when parameters are added")
	})
}
