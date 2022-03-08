// Copyright 2016-2022, Pulumi Corporation.
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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func TestDetectProjectAndPath(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "projecttest")
	assert.NoError(t, err)
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	defer func() { err := os.Chdir(cwd); assert.NoError(t, err) }()
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)

	yamlPath := filepath.Join(tmpDir, "Pulumi.yaml")
	yamlContents :=
		"name: some_project\ndescription: Some project\nruntime: nodejs\n"

	err = os.WriteFile(yamlPath, []byte(yamlContents), 0600)
	assert.NoError(t, err)

	project, path, err := DetectProjectAndPath()
	assert.NoError(t, err)
	assert.Equal(t, yamlPath, path)
	assert.Equal(t, tokens.PackageName("some_project"), project.Name)
	assert.Equal(t, "Some project", *project.Description)
	assert.Equal(t, "nodejs", project.Runtime.name)
}

func TestProjectStackPath(t *testing.T) {
	expectedPath := func(expectedPath string) func(t *testing.T, projectDir, path string, err error) {
		return func(t *testing.T, projectDir, path string, err error) {
			assert.NoError(t, err)
			assert.Equal(t, filepath.Join(projectDir, expectedPath), path)
		}
	}

	tests := []struct {
		name         string
		yamlContents string
		validate     func(t *testing.T, projectDir, path string, err error)
	}{{
		"WithoutStacksDirectory",
		"name: some_project\ndescription: Some project\nruntime: nodejs\n",
		expectedPath("Pulumi.my_stack.yaml"),
	}, {
		"WithStacksDirectory",
		"name: some_project\ndescription: Some project\nruntime: nodejs\nstacksDirectory: stacks\n",
		expectedPath(filepath.Join("stacks", "Pulumi.my_stack.yaml")),
	}, {
		"WithConfig",
		"name: some_project\ndescription: Some project\nruntime: nodejs\nconfig: stacks\n",
		expectedPath(filepath.Join("stacks", "Pulumi.my_stack.yaml")),
	}, {
		"WithBoth",
		"name: some_project\ndescription: Some project\nruntime: nodejs\nconfig: stacks\nstacksDirectory: stacks\n",
		func(t *testing.T, projectDir, path string, err error) {
			assert.Error(t, err)
		},
	}}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "projecttest")
			assert.NoError(t, err)
			cwd, err := os.Getwd()
			assert.NoError(t, err)
			defer func() { err := os.Chdir(cwd); assert.NoError(t, err) }()
			err = os.Chdir(tmpDir)
			assert.NoError(t, err)

			err = os.WriteFile(
				filepath.Join(tmpDir, "Pulumi.yaml"),
				[]byte(tt.yamlContents),
				0600)
			assert.NoError(t, err)

			path, err := DetectProjectStackPath("my_stack")
			tt.validate(t, tmpDir, path, err)
		})
	}
}
