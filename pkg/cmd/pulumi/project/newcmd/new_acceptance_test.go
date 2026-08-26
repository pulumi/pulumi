// Copyright 2016, Pulumi Corporation.
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

// TestRegress13774 checks that you can run `pulumi new` on an existing project as described in the
// Pulumi Cloud new project instructions.

//nolint:paralleltest // changes directory for process
func TestRegress13774(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	orgName := ""
	projectName := genUniqueName(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	args := newArgs{
		interactive:       false,
		yes:               true,
		stack:             strings.Join([]string{orgName, projectName, "some-stack"}, "/"),
		secretsProvider:   "default",
		description:       "description", // Needs special escaping for YAML
		templateNameOrURL: "typescript",
		force:             true,
		languageTemplate:  languageTemplateMock,
	}

	// Create new project.
	removeStackOnCleanup(t, tempdir, args.stack)
	err := runNew(t.Context(), args)
	require.NoError(t, err)

	// Create new stack on an existing project.
	args.stack = strings.Join([]string{orgName, projectName, "dev"}, "/")
	removeStackOnCleanup(t, tempdir, args.stack)
	err = runNew(t.Context(), args)
	require.NoError(t, err, "should be able to run `pulumi new` successfully on an existing project")
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

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
		languageTemplate:  languageTemplateMock,
	}

	removeStackOnCleanup(t, tempdir, orgStackName)
	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithNumericName(t *testing.T) {
	t.Skip("https://github.com/pulumi/pulumi/issues/20410")
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

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
		languageTemplate:  languageTemplateMock,
	}

	removeStackOnCleanup(t, tempdir, orgStackName)
	err := runNew(t.Context(), args)
	require.NoError(t, err)

	p := loadProject(t, tempdir)
	require.NotNil(t, p)

	assert.Equal(t, p.Name.String(), numericProjectName)

	assert.Equal(t, fullStackName, loadStackName(t))
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	uniqueProjectName := filepath.Base(tempdir)

	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, orgStackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
		languageTemplate:  languageTemplateMock,
	}

	removeStackOnCleanup(t, tempdir, orgStackName)
	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithDefaultName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	defaultProjectName := filepath.Base(tempdir)

	args := newArgs{
		interactive:       true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
		languageTemplate:  languageTemplateMock,
	}

	removeStackOnCleanup(t, tempdir, stackName)
	err := runNew(t.Context(), args)
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
}

func TestCreatingProjectWithPulumiBackendURL(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)
	ctx := t.Context()

	b, err := backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, nil, display.Options{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(b.URL(), "https://app.pulumi.com"))

	backendDir := t.TempDir()

	// Now override to local filesystem backend
	backendURL := "file://" + filepath.ToSlash(backendDir)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "how now brown cow")
	t.Setenv(env.BackendURL.Var().Name(), backendURL)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	defaultProjectName := filepath.Base(tempdir)

	args := newArgs{
		interactive:       true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
		languageTemplate:  languageTemplateMock,
	}

	require.NoError(t, runNew(t.Context(), args))
	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
	// Expect the stack directory to have a checkpoint file for the stack.
	_, err = os.Stat(filepath.Join(
		backendDir, workspace.BookkeepingDir, pkgWorkspace.StackDir, defaultProjectName, stackName+".json",
	))
	require.NoError(t, err)

	b, err = backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, nil, display.Options{})
	require.NoError(t, err)
	assert.Equal(t, backendURL, b.URL())
}

//nolint:paralleltest // changes directory for process
func TestRunNewYesWithTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping template download test in short mode")
	}

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	args := newArgs{
		yes:               true,
		interactive:       false,
		templateNameOrURL: "yaml",
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		generateOnly:      true,
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)
	proj := loadProject(t, args.dir)
	require.Equal(t, "yaml", proj.Runtime.Name())
}

func TestRunNewErrorsOnRetiredAIFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args newArgs
	}{
		{name: "ai prompt only", args: newArgs{aiPrompt: "an s3 bucket"}},
		{name: "language only", args: newArgs{aiLanguage: "yaml"}},
		{name: "both flags", args: newArgs{aiPrompt: "an s3 bucket", aiLanguage: "yaml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := runNew(t.Context(), tt.args)
			require.EqualError(t, err, aiRetiredMessage)
		})
	}
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
	require.NoError(t, err)
	proj, err := workspace.LoadProject(path)
	require.NoError(t, err)
	return proj
}

func currentUser(t *testing.T) string {
	ctx := t.Context()
	b, err := backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager, nil, display.Options{})
	require.NoError(t, err)
	currentUser, _, _, err := b.CurrentUser()
	require.NoError(t, err)
	return currentUser
}

func loadStackName(t *testing.T) string {
	w, err := pkgWorkspace.Instance.New("")
	require.NoError(t, err)
	backendURL, err := pkgWorkspace.GetCurrentCloudURL(pkgWorkspace.Instance, env.Global(), nil)
	require.NoError(t, err)
	name, _ := w.Settings().StackForBackend(backendURL)
	return name
}

// removeStackOnCleanup arranges for the named stack to be removed when the test ends. Register it
// before the assertion guarding creation: `pulumi new` can create the stack and then fail.
func removeStackOnCleanup(t *testing.T, dir, name string) {
	t.Helper()
	t.Cleanup(func() {
		//nolint:usetesting // t.Context is already canceled by the time cleanup functions run.
		ctx := context.Background()
		project := loadProject(t, dir)
		b, err := backend.CurrentBackend(ctx, pkgWorkspace.Instance, backend.DefaultLoginManager,
			project, display.Options{})
		require.NoError(t, err)
		ref, err := b.ParseStackReference(name)
		require.NoError(t, err)
		stack, err := b.GetStack(ctx, ref)
		require.NoError(t, err)
		if stack == nil {
			return
		}
		_, err = b.RemoveStack(ctx, stack, true /*force*/, false /*removeBackups*/)
		require.NoError(t, err)
	})
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
