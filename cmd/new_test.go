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
package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestCreatingStackWithArgsSpecifiedName(t *testing.T) {
	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	var args = newArgs{
		interactive:       false,
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

func TestCreatingStackWithPromptedName(t *testing.T) {
	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	uniqueProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		name:              uniqueProjectName,
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
	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	var args = newArgs{
		interactive:       false,
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

func TestCreatingProjectWithDefaultName(t *testing.T) {
	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	defaultProjectName := filepath.Base(tempdir)

	var args = newArgs{
		interactive:       true,
		prompt:            promptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
		yes:               true,
	}

	err := runNew(args)
	assert.NoError(t, err)

	removeStack(t, "dev")

	proj := loadProject(t, tempdir)
	assert.Equal(t, defaultProjectName, proj.Name.String())
}

func TestCreatingProjectWithArgsSpecifiedName(t *testing.T) {
	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))
	uniqueProjectName := filepath.Base(tempdir) + "test"

	var args = newArgs{
		interactive:       false,
		name:              uniqueProjectName,
		prompt:            promptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.NoError(t, err)

	removeStack(t, "dev")

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

func TestCreatingProjectWithPromptedName(t *testing.T) {
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
		prompt:            promptMock("not#valid", ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name may only contain")
}

func TestInvalidTemplateName(t *testing.T) {
	t.Run("NoTemplateSpecified", func(t *testing.T) {
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		var args = newArgs{
			secretsProvider:   "default",
			templateNameOrURL: "",
		}

		err := runNew(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "no template selected")
	})

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		// A template that will never exist.
		template := "this-is-not-the-template-youre-looking-for"

		var args = newArgs{
			secretsProvider:   "default",
			templateNameOrURL: template,
		}

		err := runNew(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
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
	_, err = b.RemoveStack(context.Background(), ref, false)
	assert.NoError(t, err)
}
