// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // uses shared state in pulumi dir
func TestRetrieveNonExistingTemplate(t *testing.T) {
	tests := []struct {
		testName     string
		templateKind TemplateKind
	}{
		{
			testName:     "TemplateKindPulumiProject",
			templateKind: TemplateKindPulumiProject,
		},
		{
			testName:     "TemplateKindPolicyPack",
			templateKind: TemplateKindPolicyPack,
		},
	}

	templateName := "not-the-template-that-exists-in-pulumi-repo-nor-on-disk"
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			_, err := RetrieveTemplates(context.Background(), templateName, false, tt.templateKind)
			assert.ErrorIs(t, err, TemplateNotFoundError{})
			assert.ErrorIs(t, err, &TemplateNotFoundError{})
			assert.EqualError(t, err, fmt.Sprintf("template '%s' not found", templateName))
		})
	}
}

//nolint:paralleltest // uses shared state in pulumi dir
func TestRetrieveNonExistingTemplateSimilar(t *testing.T) {
	tests := []struct {
		templateName string
		expected     string
	}{
		{
			templateName: "aws-goo",
			expected: `template 'aws-goo' not found

Did you mean this?
	aws-go
`,
		},
		{
			templateName: "aws-xsharp",
			expected: `template 'aws-xsharp' not found

Did you mean this?
	aws-csharp
	aws-fsharp
`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.templateName, func(t *testing.T) {
			t.Parallel()

			_, err := RetrieveTemplates(context.Background(), tt.templateName, false, TemplateKindPulumiProject)
			assert.ErrorIs(t, err, TemplateNotFoundError{})
			assert.ErrorIs(t, err, &TemplateNotFoundError{})
			assert.EqualError(t, err, tt.expected)
		})
	}
}

//nolint:paralleltest // uses shared state in pulumi dir
func TestRetrieveStandardTemplate(t *testing.T) {
	tests := []struct {
		testName     string
		templateKind TemplateKind
		templateName string
	}{
		{
			testName:     "TemplateKindPulumiProject",
			templateKind: TemplateKindPulumiProject,
			templateName: "typescript",
		},
		{
			testName:     "TemplateKindPolicyPack",
			templateKind: TemplateKindPolicyPack,
			templateName: "aws-typescript",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			repository, err := RetrieveTemplates(context.Background(), tt.templateName, false, tt.templateKind)
			assert.NoError(t, err)
			assert.Equal(t, false, repository.ShouldDelete)

			// Root should point to Pulumi templates directory
			// (e.g. ~/.pulumi/templates or ~/.pulumi/templates-policy)
			templateDir, _ := GetTemplateDir(tt.templateKind)
			assert.Equal(t, templateDir, repository.Root)

			// SubDirectory should be a direct subfolder of Root with the name of the template
			expectedPath := filepath.Join(repository.Root, tt.templateName)
			assert.Equal(t, expectedPath, repository.SubDirectory)
		})
	}
}

//nolint:paralleltest // uses shared state in pulumi dir
func TestRetrieveHttpsTemplate(t *testing.T) {
	tests := []struct {
		testName        string
		templateKind    TemplateKind
		templateURL     string
		yamlFile        string
		expectedSubPath []string
	}{
		{
			testName:        "TemplateKindPulumiProject",
			templateKind:    TemplateKindPulumiProject,
			templateURL:     "https://github.com/pulumi/pulumi/tree/test-examples/examples/minimal",
			yamlFile:        "Pulumi.yaml",
			expectedSubPath: []string{"examples", "minimal"},
		},
		{
			testName:        "TemplateKindPolicyPack",
			templateKind:    TemplateKindPolicyPack,
			templateURL:     "https://github.com/pulumi/pulumi/tree/test-examples/examples/policy-packs/aws-ts-advanced",
			yamlFile:        "PulumiPolicy.yaml",
			expectedSubPath: []string{"examples", "policy-packs", "aws-ts-advanced"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			repository, err := RetrieveTemplates(context.Background(), tt.templateURL, false, tt.templateKind)
			assert.NoError(t, err)
			assert.Equal(t, true, repository.ShouldDelete)

			// Root should point to a subfolder of a Temp Dir
			tempDir := os.TempDir()
			pattern := filepath.Join(tempDir, "*")
			matched, _ := filepath.Match(pattern, repository.Root)
			assert.True(t, matched)

			// SubDirectory follows the path of the template in the Git repo
			pathElements := append([]string{repository.Root}, tt.expectedSubPath...)
			expectedPath := filepath.Join(pathElements...)
			assert.Equal(t, expectedPath, repository.SubDirectory)

			// SubDirectory should exist and contain the template files
			yamlPath := filepath.Join(repository.SubDirectory, tt.yamlFile)
			_, err = os.Stat(yamlPath)
			assert.NoError(t, err)

			// Clean Up
			err = repository.Delete()
			assert.NoError(t, err)
		})
	}
}

func TestRetrieveHttpsTemplateOffline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testName     string
		templateKind TemplateKind
		templateURL  string
	}{
		{
			testName:     "TemplateKindPulumiProject",
			templateKind: TemplateKindPulumiProject,
			templateURL:  "https://github.com/pulumi/pulumi-aws/tree/master/examples/minimal",
		},
		{
			testName:     "TemplateKindPolicyPack",
			templateKind: TemplateKindPolicyPack,
			templateURL:  "https://github.com/pulumi/examples/tree/master/policy-packs/aws-ts-advanced",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			_, err := RetrieveTemplates(context.Background(), tt.templateURL, true, tt.templateKind)
			assert.EqualError(t, err, fmt.Sprintf("cannot use %s offline", tt.templateURL))
		})
	}
}

//nolint:paralleltest // uses shared state in pulumi dir
func TestRetrieveFileTemplate(t *testing.T) {
	tests := []struct {
		testName     string
		templateKind TemplateKind
	}{
		{
			testName:     "TemplateKindPulumiProject",
			templateKind: TemplateKindPulumiProject,
		},
		{
			testName:     "TemplateKindPolicyPack",
			templateKind: TemplateKindPolicyPack,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			repository, err := RetrieveTemplates(context.Background(), ".", false, tt.templateKind)
			assert.NoError(t, err)
			assert.Equal(t, false, repository.ShouldDelete)

			// Both Root and SubDirectory just point to the (existing) specified folder
			assert.Equal(t, ".", repository.Root)
			assert.Equal(t, ".", repository.SubDirectory)
		})
	}
}

//nolint:paralleltest
func TestCopyTemplateFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName    string
		directories []string
		files       []string
	}{
		{
			testName: "FlatProject",
			files:    []string{"main.go", "Pulumi.yaml", "Pulumi.dev.yaml"},
		},
		{
			testName:    "NestedProject",
			directories: []string{"src"},
			files:       []string{"src/main.go", "Pulumi.yaml", "Pulumi.dev.yaml"},
		},
	}

	setupTestData := func(t *testing.T, testDataDir string, files []string, directories []string) (string, string) {
		err := os.MkdirAll(testDataDir, 0o700)
		assert.NoError(t, err)

		projectDir := testDataDir + "/project"
		err = os.MkdirAll(projectDir, 0o700)
		assert.NoError(t, err)

		copyDestDir := testDataDir + "/tmp"
		err = os.MkdirAll(copyDestDir, 0o700)
		assert.NoError(t, err)

		for _, dirName := range directories {
			err := os.MkdirAll(projectDir+"/"+dirName, 0o700)
			assert.NoError(t, err)
		}

		for _, fileName := range files {
			err := os.WriteFile(projectDir+"/"+fileName, []byte("testing"), 0o600)
			assert.NoError(t, err)
		}

		return projectDir, copyDestDir
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Copy "+tt.testName+": force=false", func(t *testing.T) {
			testDataDir := "CopyTemplateFilesTestData-Copy"

			defer func() {
				err := os.RemoveAll(testDataDir)
				assert.NoError(t, err)
			}()

			projectDir, copyDestDir := setupTestData(t, testDataDir, tt.files, tt.directories)

			err := CopyTemplateFiles(projectDir, copyDestDir, false, "testProjectName", "testProjectDescription")
			assert.NoError(t, err)
		})
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Copy "+tt.testName+": force=true", func(t *testing.T) {
			testDataDir := "CopyTemplateFilesTestData-CopyForce"

			defer func() {
				err := os.RemoveAll(testDataDir)
				assert.NoError(t, err)
			}()

			projectDir, copyDestDir := setupTestData(t, testDataDir, tt.files, tt.directories)

			err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
			assert.NoError(t, err)
		})
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Overwrite "+tt.testName+": force=false", func(t *testing.T) {
			testDataDir := "CopyTemplateFilesTestData-Overwrite"

			defer func() {
				err := os.RemoveAll(testDataDir)
				assert.NoError(t, err)
			}()

			projectDir, copyDestDir := setupTestData(t, testDataDir, tt.files, tt.directories)

			err := CopyTemplateFiles(projectDir, copyDestDir, false, "testProjectName", "testProjectDescription")
			assert.NoError(t, err)
			// copy the same files again to test overwriting - expect error
			err = CopyTemplateFiles(projectDir, copyDestDir, false, "testProjectName", "testProjectDescription")
			assert.Error(t, err)
		})
	}

	for _, tt := range tests {
		tt := tt
		t.Run("Overwrite "+tt.testName+": force=true", func(t *testing.T) {
			testDataDir := "CopyTemplateFilesTestData-OverwriteForce"

			defer func() {
				err := os.RemoveAll(testDataDir)
				assert.NoError(t, err)
			}()

			projectDir, copyDestDir := setupTestData(t, testDataDir, tt.files, tt.directories)

			err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
			assert.NoError(t, err)
			// copy the same files again to test overwriting - expect no error with force
			err = CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
			assert.NoError(t, err)
		})
	}

	t.Run("Overwrite directory over file: force=false", func(t *testing.T) {
		testDataDir := "CopyTemplateFilesTestData-OverwriteDirectoryOverFile"

		defer func() {
			err := os.RemoveAll(testDataDir)
			assert.NoError(t, err)
		}()

		directories := []string{"src"}
		files := []string{"src/main.go", "Pulumi.yaml", "Pulumi.dev.yaml"}

		projectDir, copyDestDir := setupTestData(t, testDataDir, files, directories)

		err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)

		// change the src directory in the destination dir to a file
		err = os.RemoveAll(copyDestDir + "/src")
		assert.NoError(t, err)

		err = os.WriteFile(copyDestDir+"/src", []byte("testing"), 0o600)
		assert.NoError(t, err)

		// copy the same files again to test overwriting - expect error
		err = CopyTemplateFiles(projectDir, copyDestDir, false, "testProjectName", "testProjectDescription")
		assert.Error(t, err)
	})

	t.Run("Overwrite directory over file: force=true", func(t *testing.T) {
		testDataDir := "CopyTemplateFilesTestData-OverwriteDirectoryOverFileForce"

		defer func() {
			err := os.RemoveAll(testDataDir)
			assert.NoError(t, err)
		}()

		directories := []string{"src"}
		files := []string{"src/main.go", "Pulumi.yaml", "Pulumi.dev.yaml"}

		projectDir, copyDestDir := setupTestData(t, testDataDir, files, directories)

		err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)

		// change the src directory in the destination dir to a file
		err = os.RemoveAll(copyDestDir + "/src")
		assert.NoError(t, err)

		err = os.WriteFile(copyDestDir+"/src", []byte("testing"), 0o600)
		assert.NoError(t, err)

		// copy the same files again to test overwriting - expect no error with force
		err = CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)
	})

	t.Run("Overwrite file over empty directory: force=false", func(t *testing.T) {
		testDataDir := "CopyTemplateFilesTestData-OverwriteFileOverEmptyDirectory"

		defer func() {
			err := os.RemoveAll(testDataDir)
			assert.NoError(t, err)
		}()

		directories := []string{"src"}
		files := []string{"src/main.go", "Pulumi.yaml", "Pulumi.dev.yaml"}

		projectDir, copyDestDir := setupTestData(t, testDataDir, files, directories)

		err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)

		// change the Pulumi.dev.yaml file in the destination dir to a dir
		err = os.RemoveAll(copyDestDir + "/Pulumi.dev.yaml")
		assert.NoError(t, err)

		err = os.Mkdir(copyDestDir+"/Pulumi.dev.yaml", 0o700)
		assert.NoError(t, err)

		// copy the same files again to test overwriting - expect error
		err = CopyTemplateFiles(projectDir, copyDestDir, false, "testProjectName", "testProjectDescription")
		assert.Error(t, err)
	})

	t.Run("Overwrite file over empty directory: force=true", func(t *testing.T) {
		testDataDir := "CopyTemplateFilesTestData-OverwriteFileOverEmptyDirectoryForce"

		defer func() {
			err := os.RemoveAll(testDataDir)
			assert.NoError(t, err)
		}()

		directories := []string{"src"}
		files := []string{"src/main.go", "Pulumi.yaml", "Pulumi.dev.yaml"}

		projectDir, copyDestDir := setupTestData(t, testDataDir, files, directories)

		err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)

		// change the Pulumi.dev.yaml file in the destination dir to a dir
		err = os.RemoveAll(copyDestDir + "/Pulumi.dev.yaml")
		assert.NoError(t, err)

		err = os.Mkdir(copyDestDir+"/Pulumi.dev.yaml", 0o700)
		assert.NoError(t, err)

		// copy the same files again to test overwriting - expect no error with force
		err = CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)
	})

	t.Run("Overwrite file over non-empty directory: force=true", func(t *testing.T) {
		testDataDir := "CopyTemplateFilesTestData-OverwriteFileOverNonEmptyDirectoryWithForce"

		defer func() {
			err := os.RemoveAll(testDataDir)
			assert.NoError(t, err)
		}()

		directories := []string{"src"}
		files := []string{"src/main.go", "Pulumi.yaml", "Pulumi.dev.yaml"}

		projectDir, copyDestDir := setupTestData(t, testDataDir, files, directories)

		err := CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)

		// change the Pulumi.dev.yaml file in the destination dir to a dir
		err = os.RemoveAll(copyDestDir + "/Pulumi.dev.yaml")
		assert.NoError(t, err)

		err = os.Mkdir(copyDestDir+"/Pulumi.dev.yaml", 0o700)
		assert.NoError(t, err)

		// add a file to the dir
		err = os.WriteFile(copyDestDir+"/Pulumi.dev.yaml/README.md", []byte("testing"), 0o600)
		assert.NoError(t, err)

		// copy the same files again to test overwriting - expect no error with force
		err = CopyTemplateFiles(projectDir, copyDestDir, true, "testProjectName", "testProjectDescription")
		assert.NoError(t, err)
	})
}
