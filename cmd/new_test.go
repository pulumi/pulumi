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
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	pul_testing "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestCreatingProjectWithSpecifiedName(t *testing.T) {
	e := pul_testing.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	projectName := filepath.Base(e.RootPath)

	var args = newArgs{
		interactive:       false,
		dir:               e.RootPath,
		name:              projectName,
		templateNameOrURL: "typescript",
		secretsProvider:   "default",
		force:             true,
		prompt:            promptForValue,
	}

	assert.NoError(t, os.Chdir(e.CWD))
	err := runNew(args)
	assert.NoError(t, err)

	path, err := workspace.DetectProjectPathFrom(e.RootPath)
	assert.NoError(t, err)
	proj, err := workspace.LoadProject(path)
	assert.NoError(t, err)

	assert.Equal(t, projectName, proj.Name.String())
}

func TestCreatingProjectWithEnteredName(t *testing.T) {
	e := pul_testing.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	projectName := filepath.Base(e.RootPath)

	promptMock := func(
		yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options) (string, error) {
		return projectName, nil
	}

	var args = newArgs{
		interactive:       false,
		dir:               e.RootPath,
		name:              projectName,
		templateNameOrURL: "typescript",
		prompt:            promptMock,
		secretsProvider:   "default",
		force:             true,
	}

	assert.NoError(t, os.Chdir(e.CWD))
	err := runNew(args)
	assert.NoError(t, err)

	path, err := workspace.DetectProjectPathFrom(e.RootPath)
	assert.NoError(t, err)
	proj, err := workspace.LoadProject(path)
	assert.NoError(t, err)

	assert.Equal(t, projectName, proj.Name.String())
}

func TestCreatingProjectWithExistingEnteredNameFails(t *testing.T) {
	e := pul_testing.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	projectName := "test_project"

	promptMock := func(
		yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options) (string, error) {
		if valueType == "project name" {
			err := isValidFn(projectName)
			return projectName, err
		}
		return "", nil
	}

	backendInstance = &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, name string) (bool, error) {
			return name == projectName, nil
		},
	}

	var args = newArgs{
		dir:               e.RootPath,
		force:             true,
		interactive:       true,
		prompt:            promptMock,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	assert.NoError(t, os.Chdir(e.CWD))
	err := runNew(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project with this name already exists")
}
