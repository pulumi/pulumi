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

package packagecmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPackageNewDistributablePython(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          "MyTestComponent",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
	require.NoError(t, err)

	expectedFiles := []string{
		"MyTestComponent.py",
		"__main__.py",
		"PulumiPlugin.yaml",
		"requirements.txt",
		"README.md",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		assert.FileExists(t, filePath, "Expected file %s to exist", file)
	}

	componentFile := filepath.Join(tempDir, "MyTestComponent.py")
	content, err := os.ReadFile(componentFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "class MyTestComponent(pulumi.ComponentResource)")
	assert.Contains(t, string(content), "'components:index:MyTestComponent'")
}

func TestRunPackageNewLocalPython(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          "MyLocalComponent",
		dir:           tempDir,
		componentType: "local",
	}

	err := runPackageNew(args)
	require.NoError(t, err)

	componentFile := filepath.Join(tempDir, "MyLocalComponent.py")
	assert.FileExists(t, componentFile)

	mainFile := filepath.Join(tempDir, "__main__.py")
	assert.NoFileExists(t, mainFile, "Local component should not have __main__.py")

	pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
	assert.NoFileExists(t, pluginFile, "Local component should not have PulumiPlugin.yaml")

	content, err := os.ReadFile(componentFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "'pkg:MyLocalComponent:MyLocalComponent'")
}

func TestRunPackageNewDistributableTypeScript(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "typescript",
		name:          "MyTsComponent",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
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

	packageFile := filepath.Join(tempDir, "package.json")
	packageContent, err := os.ReadFile(packageFile)
	require.NoError(t, err)
	assert.Contains(t, string(packageContent), `"name": "MyTsComponent"`)
}

func TestRunPackageNewDistributableGo(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "go",
		name:          "MyGoComponent",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
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
	assert.Contains(t, string(content), `"components:index:MyGoComponent"`)
}

func TestRunPackageNewWithShortLanguageName(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "py",
		name:          "TestComponent",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
	require.NoError(t, err)

	componentFile := filepath.Join(tempDir, "TestComponent.py")
	assert.FileExists(t, componentFile)
}

func TestRunPackageNewWithInvalidLanguage(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "rust",
		name:          "TestComponent",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language: rust")
}

func TestRunPackageNewWithInvalidComponentType(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          "TestComponent",
		dir:           tempDir,
		componentType: "invalid-type",
	}

	err := runPackageNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid component type")
}

func TestRunPackageNewCreatesDirectoryIfNotExists(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "new-package-dir")

	args := &packageNewArgs{
		language:      "typescript",
		name:          "TestComponent",
		dir:           targetDir,
		componentType: "local",
	}

	err := runPackageNew(args)
	require.NoError(t, err)

	assert.DirExists(t, targetDir)

	indexFile := filepath.Join(targetDir, "index.ts")
	assert.FileExists(t, indexFile)
}

func TestRunPackageNewWithMultiLanguageAlias(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          "TestComponent",
		dir:           tempDir,
		componentType: "multi-language",
	}

	err := runPackageNew(args)
	require.NoError(t, err)

	pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
	assert.FileExists(t, pluginFile, "multi-language should create distributable component")
}

func TestRunPackageNewWithSingleLanguageAlias(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          "TestComponent",
		dir:           tempDir,
		componentType: "single-language",
	}

	err := runPackageNew(args)
	require.NoError(t, err)

	pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
	assert.NoFileExists(t, pluginFile, "single-language should create local component")
}

func TestRunPackageNewAllLanguagesDistributable(t *testing.T) {
	t.Parallel()

	languages := []string{"python", "typescript", "go", "csharp", "java", "yaml"}

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			args := &packageNewArgs{
				language:      lang,
				name:          "TestComponent",
				dir:           tempDir,
				componentType: "distributable",
			}

			err := runPackageNew(args)
			require.NoError(t, err, "Failed to create %s distributable component", lang)

			if lang != "yaml" && lang != "yml" {
				pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
				assert.FileExists(t, pluginFile, "Expected PulumiPlugin.yaml for %s", lang)

				content, err := os.ReadFile(pluginFile)
				require.NoError(t, err)
				assert.Contains(t, string(content), "name: TestComponent")
			} else {
				pulumiFile := filepath.Join(tempDir, "Pulumi.yaml")
				assert.FileExists(t, pulumiFile, "Expected Pulumi.yaml for YAML component")
			}
		})
	}
}

func TestRunPackageNewAllLanguagesLocal(t *testing.T) {
	t.Parallel()

	languages := []string{"python", "typescript", "go", "csharp", "java", "yaml"}

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			args := &packageNewArgs{
				language:      lang,
				name:          "TestComponent",
				dir:           tempDir,
				componentType: "local",
			}

			err := runPackageNew(args)
			require.NoError(t, err, "Failed to create %s local component", lang)

			pluginFile := filepath.Join(tempDir, "PulumiPlugin.yaml")
			assert.NoFileExists(t, pluginFile, "Local component should not have PulumiPlugin.yaml for %s", lang)
		})
	}
}

func TestRunPackageNewInvalidNameDot(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          ".",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid component name")
}

func TestRunPackageNewInvalidNameDoubleDot(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	args := &packageNewArgs{
		language:      "python",
		name:          "..",
		dir:           tempDir,
		componentType: "distributable",
	}

	err := runPackageNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid component name")
}
