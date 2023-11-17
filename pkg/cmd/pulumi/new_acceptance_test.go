// Copyright 2016-2023, Pulumi Corporation.
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
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
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

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		description:       "foo: bar", // Needs special escaping for YAML
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
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

	args := newArgs{
		interactive:       false,
		yes:               true,
		name:              numericProjectName, // Should be serialized as a string.
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "yaml",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	p := loadProject(t, tempdir)
	assert.NotNil(t, p)

	assert.Equal(t, p.Name.String(), numericProjectName)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithDefaultName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	defaultProjectName := filepath.Base(tempdir)

	args := newArgs{
		interactive:       true,
		prompt:            promptForValue,
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

	b, err := currentBackend(ctx, nil, display.Options{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(b.URL(), "https://app.pulumi.com"))

	fileStateDir := t.TempDir()

	// Now override to local filesystem backend
	backendURL := "file://" + filepath.ToSlash(fileStateDir)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "how now brown cow")
	t.Setenv(workspace.PulumiBackendURLEnvVar, backendURL)

	backendInstance = nil
	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	defaultProjectName := filepath.Base(tempdir)

	args := newArgs{
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
	_, err = os.Stat(filepath.Join(
		fileStateDir, workspace.BookkeepingDir, workspace.StackDir, defaultProjectName, stackName+".json"))
	assert.NoError(t, err)

	b, err = currentBackend(ctx, nil, display.Options{})
	require.NoError(t, err)
	assert.Equal(t, backendURL, b.URL())
}

//nolint:paralleltest // changes directory for process
func TestTemplateDisplayName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir)

	// Make up a dummy template folder that sets display name
	templateDir := t.TempDir()
	err := os.Mkdir(filepath.Join(templateDir, "templateA"), 0o700)
	require.NoError(t, err)
	template := workspace.Project{
		Name:    "${PROJECT}",
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
		Template: &workspace.ProjectTemplate{
			DisplayName: "My Template",
		},
	}
	err = template.Save(filepath.Join(templateDir, "templateA", "Pulumi.yaml"))
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(templateDir, "templateB"), 0o700)
	require.NoError(t, err)
	template = workspace.Project{
		Name:    "${PROJECT}",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Template: &workspace.ProjectTemplate{
			Metadata: map[string]string{
				"test": "value",
			},
		},
	}
	err = template.Save(filepath.Join(templateDir, "templateB", "Pulumi.yaml"))
	require.NoError(t, err)

	outR, outW, err := os.Pipe()
	require.NoError(t, err)

	inR, inW, err := os.Pipe()
	require.NoError(t, err)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: templateDir,
		generateOnly:      true,
		askOpt:            survey.WithStdio(inR, outW, os.Stderr),
	}

	readPromise := promise.Run(func() ([]byte, error) {
		return io.ReadAll(outR)
	})
	_, err = inW.Write([]byte{13})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = runNew(context.Background(), args)
		assert.NoError(t, err)
	}()
	wg.Wait()
	err = outW.Close()
	require.NoError(t, err)

	read, err := readPromise.Result(context.Background())
	require.NoError(t, err)
	expected := "\x1b7\x1b[?25l\x1b8\x1b[0G\x1b[2K \rPlease choose a template (2/2 shown):\n" +
		"  [Use arrows to move, type to filter]\n" +
		"> My Template    \n" +
		"  templateB      \n" +
		"\x1b7\x1b[1A\x1b[0G\x1b[1A\x1b[0G\x1b8\x1b[?25h\x1b8\x1b[0G\x1b[2K\x1b[1A\x1b" +
		"[0G\x1b[2K\x1b[1A\x1b[0G\x1b[2K\x1b[1A\x1b[0G\x1b[2K\x1b[1A\x1b[0G\x1b[2K \r" +
		"Please choose a template (2/2 shown):\n" +
		" My Template    \n"
	assert.Equal(t, expected, string(read))

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
	assert.Equal(t, "nodejs", proj.Runtime.Name())
}

const (
	projectName = "test_project"
	stackName   = "test_stack"
)

func promptMock(name string, stackName string) promptForValueFunc {
	return func(yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options,
	) (string, error) {
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
	b, err := currentBackend(ctx, nil, display.Options{})
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
	b, err := currentBackend(ctx, project, display.Options{})
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
