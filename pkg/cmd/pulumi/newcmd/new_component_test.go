// Copyright 2016-2026, Pulumi Corporation.
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

package newcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldPythonComponent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	componentName := "test-component"

	err := ScaffoldPythonComponent(tempDir, componentName)
	require.NoError(t, err)

	expectedFiles := []string{
		"test_component.py",
		"__main__.py",
		"PulumiPlugin.yaml",
		"requirements.txt",
		"README.md",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		assert.FileExists(t, filePath, "Expected file %s to exist", file)
	}

	componentFile := filepath.Join(tempDir, "test_component.py")
	content, err := os.ReadFile(componentFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "class TestComponent(pulumi.ComponentResource)")
	assert.Contains(t, string(content), "class TestComponentArgs(TypedDict)")
	assert.Contains(t, string(content), "'components:index:TestComponent'")

	mainFile := filepath.Join(tempDir, "__main__.py")
	mainContent, err := os.ReadFile(mainFile)
	require.NoError(t, err)
	assert.Contains(t, string(mainContent), "component_provider_host")
	assert.Contains(t, string(mainContent), "from test_component import TestComponent")

	pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
	pluginContent, err := os.ReadFile(pluginFile)
	require.NoError(t, err)
	assert.Contains(t, string(pluginContent), "name: test-component")
	assert.Contains(t, string(pluginContent), "runtime: python")
	assert.Contains(t, string(pluginContent), "version: 0.1.0")
}

func TestScaffoldTypeScriptComponent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	componentName := "my-ts-component"

	err := ScaffoldTypeScriptComponent(tempDir, componentName)
	require.NoError(t, err)

	expectedFiles := []string{
		"index.ts",
		"package.json",
		"tsconfig.json",
		"PulumiPlugin.yaml",
		"README.md",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		assert.FileExists(t, filePath, "Expected file %s to exist", file)
	}

	indexFile := filepath.Join(tempDir, "index.ts")
	content, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "export class MyTsComponent extends pulumi.ComponentResource")
	assert.Contains(t, string(content), "export interface MyTsComponentArgs")
	assert.Contains(t, string(content), `"components:index:MyTsComponent"`)

	packageFile := filepath.Join(tempDir, "package.json")
	packageContent, err := os.ReadFile(packageFile)
	require.NoError(t, err)
	assert.Contains(t, string(packageContent), `"name": "my-ts-component"`)

	pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
	pluginContent, err := os.ReadFile(pluginFile)
	require.NoError(t, err)
	assert.Contains(t, string(pluginContent), "name: my-ts-component")
	assert.Contains(t, string(pluginContent), "runtime: nodejs")
}

func TestScaffoldGoComponent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	componentName := "my-go-component"

	err := ScaffoldGoComponent(tempDir, componentName)
	require.NoError(t, err)

	expectedFiles := []string{
		"component.go",
		"main.go",
		"go.mod",
		"PulumiPlugin.yaml",
		"README.md",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		assert.FileExists(t, filePath, "Expected file %s to exist", file)
	}

	componentFile := filepath.Join(tempDir, "component.go")
	content, err := os.ReadFile(componentFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "type MyGoComponent struct")
	assert.Contains(t, string(content), "type MyGoComponentArgs struct")
	assert.Contains(t, string(content), "func NewMyGoComponent")
	assert.Contains(t, string(content), `"components:index:MyGoComponent"`)

	mainFile := filepath.Join(tempDir, "main.go")
	mainContent, err := os.ReadFile(mainFile)
	require.NoError(t, err)
	assert.Contains(t, string(mainContent), "infer.NewProviderBuilder()")
	assert.Contains(t, string(mainContent), `WithName("my-go-component")`)
}

func TestToTitleCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"hello-world", "HelloWorld"},
		{"my_component", "MyComponent"},
		{"my-awesome_component", "MyAwesomeComponent"},
		{"", ""},
		{"a", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := toTitleCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScaffoldCSharpComponent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	componentName := "my-csharp-component"

	err := ScaffoldCSharpComponent(tempDir, componentName)
	require.NoError(t, err)

	expectedFiles := []string{
		"MyCsharpComponent.cs",
		"MyCsharpComponent.csproj",
		"PulumiPlugin.yaml",
		"README.md",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		assert.FileExists(t, filePath, "Expected file %s to exist", file)
	}

	csFile := filepath.Join(tempDir, "MyCsharpComponent.cs")
	content, err := os.ReadFile(csFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "public class MyCsharpComponent : ComponentResource")
	assert.Contains(t, string(content), "public class MyCsharpComponentArgs : ResourceArgs")
}

func TestScaffoldJavaComponent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	componentName := "my-java-component"

	err := ScaffoldJavaComponent(tempDir, componentName)
	require.NoError(t, err)

	javaFile := filepath.Join(tempDir, "src", "main", "java", "myjavacomponent", "MyJavaComponent.java")
	assert.FileExists(t, javaFile)

	pomFile := filepath.Join(tempDir, "pom.xml")
	assert.FileExists(t, pomFile)

	content, err := os.ReadFile(javaFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "public class MyJavaComponent extends ComponentResource")
	assert.Contains(t, string(content), "public static class MyJavaComponentArgs extends ResourceArgs")
}

func TestScaffoldYamlComponent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	componentName := "my-yaml-component"

	err := ScaffoldYamlComponent(tempDir, componentName)
	require.NoError(t, err)

	yamlFile := filepath.Join(tempDir, "Pulumi.yaml")
	assert.FileExists(t, yamlFile)

	readmeFile := filepath.Join(tempDir, "README.md")
	assert.FileExists(t, readmeFile)

	content, err := os.ReadFile(yamlFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: my-yaml-component")
	assert.Contains(t, string(content), "runtime: yaml")
}
