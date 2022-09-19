// Copyright 2016-2022, Pulumi Corporation.
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
package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/shared"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func chdir(t *testing.T, dir string) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(dir)) // Set directory
	t.Cleanup(func() {
		assert.NoError(t, os.Chdir(cwd)) // Restore directory
		restoredDir, err := os.Getwd()
		assert.NoError(t, err)
		assert.Equal(t, cwd, restoredDir)
	})
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	chdir(t, tempdir)

	var args = newArgs{
		interactive:       false,
		yes:               true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithDefaultName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	chdir(t, tempdir)
	defaultProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	removeStack(t, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
}

//nolint:paralleltest // mutates environment variables
func TestCreatingProjectWithPulumiBackendURL(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)
	ctx := context.Background()

	b, err := shared.CurrentBackend(ctx, display.Options{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(b.URL(), "https://app.pulumi.com"))

	fileStateDir, _ := ioutil.TempDir("", "local-state-dir")
	defer os.RemoveAll(fileStateDir)

	// Now override to local filesystem backend
	backendURL := "file://" + filepath.ToSlash(fileStateDir)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "how now brown cow")
	t.Setenv(workspace.PulumiBackendURLEnvVar, backendURL)

	shared.BackendInstance = nil
	tempdir, _ := ioutil.TempDir("", "test-env-local")
	defer os.RemoveAll(tempdir)
	chdir(t, tempdir)
	defaultProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
	}

	assert.NoError(t, runNew(context.Background(), args))
	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
	// Expect the stack directory to have a checkpoint file for the stack.
	_, err = os.Stat(filepath.Join(fileStateDir, workspace.BookkeepingDir, workspace.StackDir, stackName+".json"))
	assert.NoError(t, err)

	b, err = shared.CurrentBackend(ctx, display.Options{})
	require.NoError(t, err)
	assert.Equal(t, backendURL, b.URL())
}

const projectName = "test_project"
const stackName = "test_stack"

func promptMock(name string, stackName string) promptForValueFunc {
	return func(yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options) (string, error) {
		if valueType == "project name" {
			err := isValidFn(name)
			return name, err
		}
		if valueType == "stack name" {
			err := isValidFn(stackName)
			return stackName, err
		}
		return defaultValue, nil
	}
}

func loadProject(t *testing.T, dir string) *workspace.Project {
	path, err := workspace.DetectProjectPathFrom(dir)
	assert.NoError(t, err)
	proj, err := workspace.LoadProject(path)
	assert.NoError(t, err)
	return proj
}

func currentUser(t *testing.T) string {
	ctx := context.Background()
	b, err := shared.CurrentBackend(ctx, display.Options{})
	assert.NoError(t, err)
	currentUser, _, err := b.CurrentUser()
	assert.NoError(t, err)
	return currentUser
}

func loadStackName(t *testing.T) string {
	w, err := workspace.New()
	assert.NoError(t, err)
	return w.Settings().Stack
}

func removeStack(t *testing.T, name string) {
	ctx := context.Background()
	b, err := shared.CurrentBackend(ctx, display.Options{})
	assert.NoError(t, err)
	ref, err := b.ParseStackReference(name)
	assert.NoError(t, err)
	stack, err := b.GetStack(context.Background(), ref)
	assert.NoError(t, err)
	_, err = b.RemoveStack(context.Background(), stack, false)
	assert.NoError(t, err)
}

func skipIfShortOrNoPulumiAccessToken(t *testing.T) {
	_, ok := os.LookupEnv("PULUMI_ACCESS_TOKEN")
	if !ok {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}
}
