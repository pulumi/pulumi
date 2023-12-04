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
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In the tests below we use temporary directories and then expect DetectProjectAndPath to return a path to
// that directory. However DetectProjectAndPath will do symlink resolution, while t.TempDir() normally does
// not. This can lead to asserts especially on macos where TmpDir will have returned /var/folders/XX, but
// after sym link resolution that is /private/var/folders/XX.
func mkTempDir(t *testing.T) string {
	tmpDir := t.TempDir()
	result, err := filepath.EvalSymlinks(tmpDir)
	assert.NoError(t, err)
	return result
}

//nolint:paralleltest // Theses test use and change the current working directory
func TestDetectProjectAndPath(t *testing.T) {
	tmpDir := mkTempDir(t)
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	defer func() { err := os.Chdir(cwd); assert.NoError(t, err) }()
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)

	yamlPath := filepath.Join(tmpDir, "Pulumi.yaml")
	yamlContents := "name: some_project\ndescription: Some project\nruntime: nodejs\n"

	err = os.WriteFile(yamlPath, []byte(yamlContents), 0o600)
	assert.NoError(t, err)

	project, path, err := DetectProjectAndPath()
	assert.NoError(t, err)
	assert.Equal(t, yamlPath, path)
	assert.Equal(t, tokens.PackageName("some_project"), project.Name)
	assert.Equal(t, "Some project", *project.Description)
	assert.Equal(t, "nodejs", project.Runtime.name)
}

//nolint:paralleltest // Theses test use and change the current working directory
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
		"WithoutStackConfigDir",
		"name: some_project\ndescription: Some project\nruntime: nodejs\n",
		expectedPath("Pulumi.my_stack.yaml"),
	}, {
		"WithStackConfigDir",
		"name: some_project\ndescription: Some project\nruntime: nodejs\nstackConfigDir: stacks\n",
		expectedPath(filepath.Join("stacks", "Pulumi.my_stack.yaml")),
	}, {
		"WithConfig",
		"name: some_project\ndescription: Some project\nruntime: nodejs\nconfig: stacks\n",
		expectedPath(filepath.Join("stacks", "Pulumi.my_stack.yaml")),
	}, {
		"WithBoth",
		"name: some_project\ndescription: Some project\nruntime: nodejs\nconfig: stacksA\nstackConfigDir: stacksB\n",
		func(t *testing.T, projectDir, path string, err error) {
			errorMsg := "Should not use both config and stackConfigDir to define the stack directory. " +
				"Use only stackConfigDir instead."
			assert.EqualError(t, err, errorMsg)
		},
	}}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := mkTempDir(t)
			cwd, err := os.Getwd()
			assert.NoError(t, err)
			defer func() { err := os.Chdir(cwd); assert.NoError(t, err) }()
			err = os.Chdir(tmpDir)
			assert.NoError(t, err)

			err = os.WriteFile(
				filepath.Join(tmpDir, "Pulumi.yaml"),
				[]byte(tt.yamlContents),
				0o600)
			assert.NoError(t, err)

			_, path, err := DetectProjectStackPath("my_stack")
			tt.validate(t, tmpDir, path, err)
		})
	}
}

//nolint:paralleltest // Theses test use and change the current working directory
func TestDetectProjectUnreadableParent(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/12481

	tmpDir := mkTempDir(t)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { err := os.Chdir(cwd); assert.NoError(t, err) }()

	// unreadable parent directory
	parentDir := filepath.Join(tmpDir, "root")
	err = os.Mkdir(parentDir, 0o300)
	require.NoError(t, err)
	// Make it readable so we can clean it up later
	defer func() { err := os.Chmod(parentDir, 0o700); assert.NoError(t, err) }()

	// readable current directory
	currentDir := filepath.Join(parentDir, "current")
	err = os.Mkdir(currentDir, 0o700)
	require.NoError(t, err)

	err = os.Chdir(currentDir)
	require.NoError(t, err)

	_, _, err = DetectProjectAndPath()
	assert.ErrorIs(t, err, ErrProjectNotFound)
}
