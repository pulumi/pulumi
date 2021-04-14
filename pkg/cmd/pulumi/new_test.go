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
package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestCreatingStackWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	var args = newArgs{
		interactive:       false,
		yes:               true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

func TestFailInInteractiveWithoutYes(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	var args = newArgs{
		interactive:       false,
		yes:               false,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.Error(t, err)
}

func TestCreatingStackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	uniqueProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

func TestCreatingStackWithArgsSpecifiedOrgName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	var args = newArgs{
		interactive:       false,
		yes:               true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             orgStackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

func TestCreatingStackWithPromptedOrgName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	uniqueProjectName := filepath.Base(tempdir)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	var args = newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, orgStackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

func TestCreatingStackWithArgsSpecifiedFullNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	// the project name and the project name in the stack name must match
	uniqueProjectName := filepath.Base(tempdir)
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), uniqueProjectName, stackName)

	var args = newArgs{
		interactive:       false,
		yes:               true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             fullStackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, stackName)
}

func TestCreatingProjectWithDefaultName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	defaultProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
	}

	err := runNew(args)
	assert.NoError(t, err)

	removeStack(t, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
}

func TestCreatingProjectWithPulumiBackendURL(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	b, err := currentBackend(display.Options{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(b.URL(), "https://app.pulumi.com"))

	fileStateDir, _ := ioutil.TempDir("", "local-state-dir")
	defer os.RemoveAll(fileStateDir)

	// Now override to local filesystem backend
	backendURL := "file://" + fileStateDir
	_ = os.Setenv("PULUMI_CONFIG_PASSPHRASE", "how now brown cow")
	_ = os.Setenv(workspace.PulumiBackendURLEnvVar, backendURL)
	defer func() {
		_ = os.Unsetenv(workspace.PulumiBackendURLEnvVar)
		_ = os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	}()

	backendInstance = nil
	tempdir, _ := ioutil.TempDir("", "test-env-local")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	defaultProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
		yes:               true,
	}

	assert.NoError(t, runNew(args))
	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
	// Expect the stack directory to have a checkpoint file for the stack.
	_, err = os.Stat(filepath.Join(fileStateDir, workspace.BookkeepingDir, workspace.StackDir, stackName+".json"))
	assert.NoError(t, err)

	b, err = currentBackend(display.Options{})
	require.NoError(t, err)
	assert.Equal(t, backendURL, b.URL())
}

func TestCreatingProjectWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	uniqueProjectName := filepath.Base(tempdir) + "test"

	var args = newArgs{
		interactive:       false,
		yes:               true,
		name:              uniqueProjectName,
		prompt:            promptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	removeStack(t, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

func TestCreatingProjectWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	uniqueProjectName := filepath.Base(tempdir) + "test"

	var args = newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	removeStack(t, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

func TestCreatingProjectWithExistingArgsSpecifiedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return name == projectName, nil
		},
	}

	var args = newArgs{
		interactive:       false,
		yes:               true,
		name:              projectName,
		prompt:            promptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project with this name already exists")
}

func TestCreatingProjectWithExistingPromptedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return name == projectName, nil
		},
	}

	var args = newArgs{
		interactive:       true,
		prompt:            promptMock(projectName, ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project with this name already exists")
}

func TestGeneratingProjectWithExistingArgsSpecifiedNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	var args = newArgs{
		generateOnly:      true,
		interactive:       false,
		yes:               true,
		name:              projectName,
		prompt:            promptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, projectName, proj.Name.String())
}

func TestGeneratingProjectWithExistingPromptedNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	var args = newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock(projectName, ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, projectName, proj.Name.String())
}

func TestGeneratingProjectWithInvalidArgsSpecifiedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	var args = newArgs{
		generateOnly:      true,
		interactive:       false,
		yes:               true,
		name:              "not#valid",
		prompt:            promptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name may only contain")
}

func TestGeneratingProjectWithInvalidPromptedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	err := runNew(newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock("not#valid", ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name may only contain")

	err = runNew(newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock("", ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name may not be empty")
}

func TestInvalidTemplateName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	t.Run("NoTemplateSpecified", func(t *testing.T) {
		t.Parallel()
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		var args = newArgs{
			interactive:       false,
			yes:               true,
			secretsProvider:   "default",
			templateNameOrURL: "",
		}

		err := runNew(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "no template selected")
	})

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		t.Parallel()
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		// A template that will never exist.
		template := "this-is-not-the-template-youre-looking-for"

		var args = newArgs{
			interactive:       false,
			yes:               true,
			secretsProvider:   "default",
			templateNameOrURL: template,
		}

		err := runNew(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		t.Parallel()

		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		var args = newArgs{
			generateOnly:      true,
			offline:           true,
			secretsProvider:   "default",
			templateNameOrURL: template,
			yes:               true,
		}

		err := runNew(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "not found")
	})
}

func TestParseConfigSuccess(t *testing.T) {
	tests := []struct {
		Array    []string
		Path     bool
		Expected config.Map
	}{
		{
			Array:    []string{},
			Expected: config.Map{},
		},
		{
			Array: []string{"my:testKey"},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue(""),
			},
		},
		{
			Array: []string{"my:testKey="},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue(""),
			},
		},
		{
			Array: []string{"my:testKey=testValue"},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{"my:testKey=test=Value"},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("test=Value"),
			},
		},
		{
			Array: []string{
				"my:testKey=testValue",
				"my:testKey=rewritten",
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("rewritten"),
			},
		},
		{
			Array: []string{
				"my:testKey=testValue",
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:test.Key=testValue",
			},
			Expected: config.Map{
				config.MustMakeKey("my", "test.Key"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:testKey=testValue",
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:0=testValue",
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "0"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:true=testValue",
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "true"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				`my:["test.Key"]=testValue`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "test.Key"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				`my:outer.inner=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "outer"): config.NewObjectValue(`{"inner":"value"}`),
			},
		},
		{
			Array: []string{
				`my:outer.inner.nested=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "outer"): config.NewObjectValue(`{"inner":{"nested":"value"}}`),
			},
		},
		{
			Array: []string{
				`my:name[0]=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "name"): config.NewObjectValue(`["value"]`),
			},
		},
		{
			Array: []string{
				`my:name[0][0]=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "name"): config.NewObjectValue(`[["value"]]`),
			},
		},
		{
			Array: []string{
				`my:servers[0].name=foo`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "servers"): config.NewObjectValue(`[{"name":"foo"}]`),
			},
		},
		{
			Array: []string{
				`my:testKey=false`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("false"),
			},
		},
		{
			Array: []string{
				`my:testKey=true`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("true"),
			},
		},
		{
			Array: []string{
				`my:testKey=10`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("10"),
			},
		},
		{
			Array: []string{
				`my:testKey=-1`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("-1"),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=false`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[false]`),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=true`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[true]`),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=10`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[10]`),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=-1`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[-1]`),
			},
		},
		{
			Array: []string{
				`my:names[0]=a`,
				`my:names[1]=b`,
				`my:names[2]=c`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "names"): config.NewObjectValue(`["a","b","c"]`),
			},
		},
		{
			Array: []string{
				`my:names[0]=a`,
				`my:names[1]=b`,
				`my:names[2]=c`,
				`my:names[0]=rewritten`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "names"): config.NewObjectValue(`["rewritten","b","c"]`),
			},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			actual, err := parseConfig(test.Array, test.Path)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, actual)
		})
	}
}

func TestSetFail(t *testing.T) {
	tests := []struct {
		Array    []string
		Expected config.Map
	}{
		{
			Array: []string{`my:[""]=value`},
		},
		{
			Array: []string{"my:[0]=value"},
		},
		{
			Array: []string{`my:name[-1]=value`},
		},
		{
			Array: []string{`my:name[1]=value`},
		},
		{
			Array: []string{`my:key.secure=value`},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			_, err := parseConfig(test.Array, true /*path*/)
			assert.Error(t, err)
		})
	}
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
	b, err := currentBackend(display.Options{})
	assert.NoError(t, err)
	currentUser, err := b.CurrentUser()
	assert.NoError(t, err)
	return currentUser
}

func loadStackName(t *testing.T) string {
	w, err := workspace.New()
	assert.NoError(t, err)
	return w.Settings().Stack
}

func removeStack(t *testing.T, name string) {
	b, err := currentBackend(display.Options{})
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
