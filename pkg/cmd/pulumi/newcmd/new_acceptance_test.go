// Copyright 2016-2024, Pulumi Corporation.
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

package newcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
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

// TestRegress13774 checks that you can run `pulumi new` on an existing project as described in the
// Pulumi Cloud new project instructions.

//nolint:paralleltest // changes directory for process
func TestRegress13774(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	orgName := ""
	projectName := genUniqueName(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	args := newArgs{
		interactive:       false,
		yes:               true,
		stack:             strings.Join([]string{orgName, projectName, "some-stack"}, "/"),
		secretsProvider:   "default",
		description:       "description", // Needs special escaping for YAML
		templateNameOrURL: "typescript",
		force:             true,
	}

	// Create new project.
	err := runNew(context.Background(), args)
	defer removeStack(t, tempdir, args.stack)
	assert.NoError(t, err)

	// Create new stack on an existing project.
	args.stack = strings.Join([]string{orgName, projectName, "dev"}, "/")
	err = runNew(context.Background(), args)
	defer removeStack(t, tempdir, args.stack)
	assert.NoError(t, err, "should be able to run `pulumi new` successfully on an existing project")
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		description:       "foo: bar", // Needs special escaping for YAML
		stack:             orgStackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, orgStackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithNumericName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	// This test requires a numeric project name.
	// Project names have to be unique or this test will fail.
	// A test may crash and leave a project behind, so we use a timestamp to try to ensure uniqueness
	// instead of a constant.
	unixTsNanos := time.Now().UnixNano()
	numericProjectName := strconv.Itoa(int(unixTsNanos))
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), numericProjectName, stackName)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	args := newArgs{
		interactive:       false,
		yes:               true,
		name:              numericProjectName, // Should be serialized as a string.
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             orgStackName,
		templateNameOrURL: "yaml",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	p := loadProject(t, tempdir)
	assert.NotNil(t, p)

	assert.Equal(t, p.Name.String(), numericProjectName)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, orgStackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir)

	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, orgStackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, orgStackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithDefaultName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	defaultProjectName := filepath.Base(tempdir)

	args := newArgs{
		interactive:       true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	removeStack(t, tempdir, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
}

//nolint:paralleltest // mutates environment variables
func TestCreatingProjectWithPulumiBackendURL(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)
	ctx := context.Background()

	b, err := backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, nil, display.Options{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(b.URL(), "https://app.pulumi.com"))

	backendDir := t.TempDir()

	// Now override to local filesystem backend
	backendURL := "file://" + filepath.ToSlash(backendDir)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "how now brown cow")
	t.Setenv(env.BackendURL.Var().Name(), backendURL)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	defaultProjectName := filepath.Base(tempdir)

	args := newArgs{
		interactive:       true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
	}

	assert.NoError(t, runNew(context.Background(), args))
	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
	// Expect the stack directory to have a checkpoint file for the stack.
	_, err = os.Stat(filepath.Join(
		backendDir, workspace.BookkeepingDir, workspace.StackDir, defaultProjectName, stackName+".json"))
	assert.NoError(t, err)

	b, err = backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, nil, display.Options{})
	require.NoError(t, err)
	assert.Equal(t, backendURL, b.URL())
}

const (
	projectName = "test_project"
	stackName   = "test_stack"
)

func promptMock(name string, stackName string) promptForValueFunc {
	return func(yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options,
	) (string, error) {
		if valueType == "Project name" {
			err := isValidFn(name)
			return name, err
		}

		if valueType == "Stack name" {
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
	b, err := backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, nil, display.Options{})
	assert.NoError(t, err)
	currentUser, _, _, err := b.CurrentUser()
	assert.NoError(t, err)
	return currentUser
}

func loadStackName(t *testing.T) string {
	w, err := workspace.New()
	require.NoError(t, err)
	return w.Settings().Stack
}

func removeStack(t *testing.T, dir, name string) {
	project := loadProject(t, dir)
	ctx := context.Background()
	b, err := backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, project, display.Options{})
	assert.NoError(t, err)
	ref, err := b.ParseStackReference(name)
	assert.NoError(t, err)
	stack, err := b.GetStack(context.Background(), ref)
	assert.NoError(t, err)
	_, err = b.RemoveStack(context.Background(), stack, false)
	assert.NoError(t, err)
}

func skipIfShortOrNoPulumiAccessToken(t *testing.T) {
	token := os.Getenv("PULUMI_ACCESS_TOKEN")
	if token == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}
}
