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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValidDefaultProjectName(t *testing.T) {
	// Valid names remain the same.
	for _, name := range getValidProjectNamePrefixes() {
		assert.Equal(t, name, getValidProjectName(name))
	}
	assert.Equal(t, "foo", getValidProjectName("foo"))
	assert.Equal(t, "foo1", getValidProjectName("foo1"))
	assert.Equal(t, "foo-", getValidProjectName("foo-"))
	assert.Equal(t, "foo-bar", getValidProjectName("foo-bar"))
	assert.Equal(t, "foo_", getValidProjectName("foo_"))
	assert.Equal(t, "foo_bar", getValidProjectName("foo_bar"))
	assert.Equal(t, "foo.", getValidProjectName("foo."))
	assert.Equal(t, "foo.bar", getValidProjectName("foo.bar"))

	// Invalid characters are left off.
	assert.Equal(t, "foo", getValidProjectName("!foo"))
	assert.Equal(t, "foo", getValidProjectName("@foo"))
	assert.Equal(t, "foo", getValidProjectName("#foo"))
	assert.Equal(t, "foo", getValidProjectName("$foo"))
	assert.Equal(t, "foo", getValidProjectName("%foo"))
	assert.Equal(t, "foo", getValidProjectName("^foo"))
	assert.Equal(t, "foo", getValidProjectName("&foo"))
	assert.Equal(t, "foo", getValidProjectName("*foo"))
	assert.Equal(t, "foo", getValidProjectName("(foo"))
	assert.Equal(t, "foo", getValidProjectName(")foo"))

	// Invalid names are replaced with a fallback name.
	assert.Equal(t, "project", getValidProjectName("!"))
	assert.Equal(t, "project", getValidProjectName("@"))
	assert.Equal(t, "project", getValidProjectName("#"))
	assert.Equal(t, "project", getValidProjectName("$"))
	assert.Equal(t, "project", getValidProjectName("%"))
	assert.Equal(t, "project", getValidProjectName("^"))
	assert.Equal(t, "project", getValidProjectName("&"))
	assert.Equal(t, "project", getValidProjectName("*"))
	assert.Equal(t, "project", getValidProjectName("("))
	assert.Equal(t, "project", getValidProjectName(")"))
	assert.Equal(t, "project", getValidProjectName("!@#$%^&*()"))
}

func TestValidateStackName(t *testing.T) {
	assert.NoError(t, ValidateStackName("alpha-beta-gamma"))
	assert.NoError(t, ValidateStackName("owner-name/alpha-beta-gamma"))

	assert.Error(t, ValidateStackName("alpha/beta/gamma"), "A stack may not contain a slash")
}

func getValidProjectNamePrefixes() []string {
	var results []string
	for ch := 'A'; ch <= 'Z'; ch++ {
		results = append(results, string(ch))
	}
	for ch := 'a'; ch <= 'z'; ch++ {
		results = append(results, string(ch))
	}
	results = append(results, "_")
	results = append(results, ".")
	return results
}

func TestRetrieveNonExistingTemplate(t *testing.T) {
	templateName := "not-the-template-that-exists-in-pulumi-repo-nor-on-disk"
	_, err := RetrieveTemplates(templateName, false)
	assert.NotNil(t, err)
}

func TestRetrieveStandardTemplate(t *testing.T) {
	templateName := "typescript"
	repository, err := RetrieveTemplates(templateName, false)
	assert.Nil(t, err)
	assert.Equal(t, false, repository.ShouldDelete)

	// Root should point to Pulumi templates directory (e.g. ~/.pulumi/templates)
	templateDir, _ := GetTemplateDir()
	assert.Equal(t, templateDir, repository.Root)

	// SubDirectory should be a direct subfolder of Root with the name of the template
	expectedPath := filepath.Join(repository.Root, templateName)
	assert.Equal(t, expectedPath, repository.SubDirectory)
}

func TestRetrieveHttpsTemplate(t *testing.T) {
	templateURL := "https://github.com/pulumi/pulumi-aws/tree/master/examples/minimal"
	repository, err := RetrieveTemplates(templateURL, false)
	assert.Nil(t, err)
	assert.Equal(t, true, repository.ShouldDelete)

	// Root should point to a subfolder of a Temp Dir
	tempDir := os.TempDir()
	pattern := filepath.Join(tempDir, "*")
	matched, _ := filepath.Match(pattern, repository.Root)
	assert.True(t, matched)

	// SubDirectory follows the path of the template in the Git repo
	expectedPath := filepath.Join(repository.Root, "examples", "minimal")
	assert.Equal(t, expectedPath, repository.SubDirectory)

	// SubDirectory should exist and contain the template files
	yamlPath := filepath.Join(repository.SubDirectory, "Pulumi.yaml")
	_, err = os.Stat(yamlPath)
	assert.Nil(t, err)

	// Clean Up
	err = repository.Delete()
	assert.Nil(t, err)
}

func TestRetrieveHttpsTemplateOffline(t *testing.T) {
	templateURL := "https://github.com/pulumi/pulumi-aws/tree/master/examples/minimal"
	_, err := RetrieveTemplates(templateURL, true)
	assert.NotNil(t, err)
}

func TestRetrieveFileTemplate(t *testing.T) {
	repository, err := RetrieveTemplates(".", false)
	assert.Nil(t, err)
	assert.Equal(t, false, repository.ShouldDelete)

	// Both Root and SubDirectory just point to the (existing) specified folder
	assert.Equal(t, ".", repository.Root)
	assert.Equal(t, ".", repository.SubDirectory)
}
