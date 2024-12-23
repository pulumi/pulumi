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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackendInstance sets the backend instance for the test and cleans it up after.
func mockBackendInstance(t *testing.T, b backend.Backend) {
	t.Cleanup(func() {
		cmdBackend.BackendInstance = nil
	})
	cmdBackend.BackendInstance = b
}

//nolint:paralleltest // changes directory for process
func TestFailInInteractiveWithoutYes(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	args := newArgs{
		interactive:       false,
		yes:               false,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.Error(t, err)
}

//nolint:paralleltest // changes directory for process
func TestFailIfProjectNameDoesNotMatch(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             "org/projectA/stack",
		name:              "projectB",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.ErrorContains(t, err, "project name (--name projectB) "+
		"and stack reference project name (--stack projectA) must be the same")
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedOrgName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             orgStackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedOrgName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	uniqueProjectName := filepath.Base(tempdir)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, orgStackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedFullNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	// the project name and the project name in the stack name must match
	uniqueProjectName := filepath.Base(tempdir)
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), uniqueProjectName, stackName)

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             fullStackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, stackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir) + "test"

	args := newArgs{
		interactive:       false,
		yes:               true,
		name:              uniqueProjectName,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	removeStack(t, tempdir, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir) + "test"

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	removeStack(t, tempdir, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestCreatingProjectWithExistingArgsSpecifiedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
	})

	args := newArgs{
		interactive:       false,
		yes:               true,
		name:              projectName,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.ErrorContains(t, err, "project with this name already exists")
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestCreatingProjectWithExistingPromptedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
	})

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(projectName, ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.ErrorContains(t, err, "Try again")
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestGeneratingProjectWithExistingArgsSpecifiedNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	args := newArgs{
		generateOnly:      true,
		interactive:       false,
		yes:               true,
		name:              projectName,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, projectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestGeneratingProjectWithExistingPromptedNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	args := newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock(projectName, ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, projectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithEmptyConfig(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/4081
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	uniqueProjectName := filepath.Base(tempdir) + "test"

	prompt := func(yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options,
	) (string, error) {
		if strings.Contains(valueType, "(aws:region)") {
			return "", nil
		}
		return defaultValue, nil
	}

	args := newArgs{
		name:              uniqueProjectName,
		stack:             stackName,
		interactive:       true,
		prompt:            prompt,
		secretsProvider:   "default",
		templateNameOrURL: "aws-typescript",
	}

	err := runNew(context.Background(), args)
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	projStack, err := workspace.LoadProjectStack(proj, filepath.Join(tempdir, "Pulumi."+stackName+".yaml"))
	require.NoError(t, err)

	assert.NotContains(t, projStack.Config, config.MustMakeKey("aws", "region"))

	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestGeneratingProjectWithInvalidArgsSpecifiedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	args := newArgs{
		generateOnly:      true,
		interactive:       false,
		yes:               true,
		name:              "not#valid",
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.ErrorContains(t, err, "project names may only contain")
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestGeneratingProjectWithInvalidPromptedNameFails(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	err := runNew(context.Background(), newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock("not#valid", ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	})
	assert.ErrorContains(t, err, "project names may only contain")

	err = runNew(context.Background(), newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock("", ""),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	})
	assert.ErrorContains(t, err, "project names may not be empty")
}

//nolint:paralleltest // changes directory for process
func TestInvalidTemplateName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	t.Run("NoTemplateSpecified", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		chdir(t, tempdir)

		args := newArgs{
			chooseTemplate:    ChooseTemplate,
			interactive:       false,
			yes:               true,
			secretsProvider:   "default",
			templateNameOrURL: "",
			templateMode:      true,
		}

		err := runNew(context.Background(), args)
		assert.ErrorContains(t, err, "no template selected")
	})

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		chdir(t, tempdir)

		// A template that will never exist.
		template := "this-is-not-the-template-youre-looking-for"

		args := newArgs{
			interactive:       false,
			yes:               true,
			secretsProvider:   "default",
			templateNameOrURL: template,
		}

		err := runNew(context.Background(), args)
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		chdir(t, tempdir)

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		args := newArgs{
			generateOnly:      true,
			offline:           true,
			secretsProvider:   "default",
			templateNameOrURL: template,
			yes:               true,
		}

		err := runNew(context.Background(), args)
		assert.ErrorContains(t, err, "not found")
	})
}

func tempProjectDir(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), genUniqueName(t))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	return dir
}

func genUniqueName(t *testing.T) string {
	t.Helper()

	var bs [8]byte
	_, err := rand.Read(bs[:])
	require.NoError(t, err)

	return "test-" + hex.EncodeToString(bs[:])
}

func TestValidateStackRefAndProjectName(t *testing.T) {
	t.Parallel()

	b := &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			parts := strings.Split(s, "/")
			switch len(parts) {
			case 1:
				return &backend.MockStackReference{
					NameV: tokens.MustParseStackName(parts[0]),
				}, nil
			case 2:
				return &backend.MockStackReference{
					ProjectV: tokens.Name(parts[0]),
					NameV:    tokens.MustParseStackName(parts[1]),
				}, nil
			case 3:
				return &backend.MockStackReference{
					ProjectV: tokens.Name(parts[1]),
					NameV:    tokens.MustParseStackName(parts[2]),
				}, nil

			default:
				return nil, fmt.Errorf("invalid stack reference %q", s)
			}
		},
	}

	tests := []struct {
		projectName string
		stackRef    string
		valid       bool
	}{
		{
			projectName: "foo",
			stackRef:    "foo",
			valid:       true,
		},
		{
			projectName: "fooo",
			stackRef:    "org/foo/dev",
			valid:       false,
		},
		{
			projectName: "",
			stackRef:    "org/foo/dev",
			valid:       true,
		},
		{
			projectName: "foo",
			stackRef:    "",
			valid:       true,
		},
		{
			projectName: "foo",
			stackRef:    "org/foo/dev",
			valid:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("project=%q/stackRef=%q", tt.projectName, tt.stackRef), func(t *testing.T) {
			t.Parallel()
			err := compareStackProjectName(b, tt.stackRef, tt.projectName)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestProjectExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, orgName string, projectName string) (bool, error) {
			type Org string
			type ProjectName string
			projects := map[Org]map[ProjectName]struct{}{
				"moolumi": {
					"my-moolumi-project": {},
				},
				"pulumi": {},
			}
			orgProjects, ok := projects[Org(orgName)]
			if !ok {
				return false, fmt.Errorf("org %s not found", orgName)
			}
			_, exists := orgProjects[ProjectName(projectName)]
			return exists, nil
		},
	}

	// Table Test
	type Project struct {
		orgName     string
		projectName string
	}
	tests := []struct {
		name   string
		give   Project
		hasErr bool
	}{
		{
			name: "project exists",
			give: Project{
				projectName: "my-moolumi-project",
				orgName:     "moolumi",
			},
			hasErr: true,
		},
		{
			name: "project exists in another org",
			give: Project{
				projectName: "my-moolumi-project",
				orgName:     "pulumi",
			},
			hasErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateProjectName(ctx, b, tt.give.orgName, tt.give.projectName, false /* generateOnly */, display.Options{})
			if tt.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:paralleltest // changes directory for process
func TestGenerateOnlyProjectCheck(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/13527, make sure various combinations of
	// project name and stack work when generateOnly is set (thus skipping backend checks).

	cases := []struct {
		name  string
		stack string
	}{
		{name: "mismatched project name", stack: "organization/boom/stack"},
		{name: "fully qualified stack name", stack: "organization/project/stack"},
		{name: "org qualified stack name", stack: "organization/stack"},
		{name: "unqualified stack name", stack: "stack"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tempdir := tempProjectDir(t)
			chdir(t, tempdir)

			args := newArgs{
				generateOnly:      true,
				interactive:       false,
				yes:               true,
				prompt:            ui.PromptForValue,
				secretsProvider:   "default",
				stack:             tt.stack,
				name:              "project",
				templateNameOrURL: "typescript",
			}

			err := runNew(context.Background(), args)
			assert.NoError(t, err)
		})
	}
}

func TestPulumiNewConflictingProject(t *testing.T) {
	t.Parallel()

	b := &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, orgName string, projectName string) (bool, error) {
			if projectName == "existing-project-name" {
				return true, nil
			}
			return false, nil
		},
	}

	assert.NoError(t,
		validateProjectNameInternal(
			context.Background(), b, "moolumi", "some-project-name", false /* generateOnly */, display.Options{},
			func(s string) error {
				assert.Fail(t, "this should not be called as this is a not a duplicate project name")
				return nil
			},
		))

	var called bool
	assert.NoError(t,
		validateProjectNameInternal(
			context.Background(), b, "moolumi", "existing-project-name", false /* generateOnly */, display.Options{},
			func(s string) error {
				called = true
				return nil
			},
		))
	assert.Truef(t, called, "expected resolution to be called with duplicate name")
}

//nolint:paralleltest // changes directory for process
func TestPulumiNewSetsTemplateTag(t *testing.T) {
	tests := []struct {
		argument string
		prompted string
		expected string
	}{
		{
			"typescript",
			"",
			"typescript",
		},
		{
			"https://github.com/pulumi/templates/tree/master/yaml?foo=bar",
			"",
			"https://github.com/pulumi/templates/tree/master/yaml",
		},
		{
			"",
			"python",
			"python",
		},
	}
	for _, tt := range tests {
		tt := tt
		name := tt.argument
		if name == "" {
			name = tt.prompted
		}
		t.Run(name, func(t *testing.T) {
			tempdir := tempProjectDir(t)
			chdir(t, tempdir)
			uniqueProjectName := filepath.Base(tempdir) + "test"

			chooseTemplateMock := func(templates []workspace.Template, opts display.Options,
			) (workspace.Template, error) {
				for _, template := range templates {
					if template.Name == tt.prompted {
						return template, nil
					}
				}
				return workspace.Template{}, errors.New("template not found")
			}

			runtimeOptionsMock := func(ctx *plugin.Context, info *workspace.ProjectRuntimeInfo,
				main string, opts display.Options, interactive, yes bool, prompt promptForValueFunc,
			) (map[string]interface{}, error) {
				return nil, nil
			}

			args := newArgs{
				interactive:          tt.prompted != "",
				generateOnly:         true,
				yes:                  true,
				templateMode:         true,
				name:                 projectName,
				prompt:               promptMock(uniqueProjectName, stackName),
				promptRuntimeOptions: runtimeOptionsMock,
				chooseTemplate:       chooseTemplateMock,
				secretsProvider:      "default",
				templateNameOrURL:    tt.argument,
			}

			err := runNew(context.Background(), args)
			assert.NoError(t, err)

			proj := loadProject(t, tempdir)
			require.NoError(t, err)
			tagsValue, has := proj.Config[apitype.PulumiTagsConfigKey]
			assert.True(t, has)
			tagsObject, ok := tagsValue.Value.(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, tt.expected, tagsObject[apitype.ProjectTemplateTag])
		})
	}
}

//nolint:paralleltest // changes directory for process
func TestPulumiPromptRuntimeOptions(t *testing.T) {
	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	runtimeOptionsMock := func(ctx *plugin.Context, info *workspace.ProjectRuntimeInfo,
		main string, opts display.Options, interactive, yes bool, prompt promptForValueFunc,
	) (map[string]interface{}, error) {
		return map[string]interface{}{"someOption": "someValue"}, nil
	}

	args := newArgs{
		interactive:          false,
		generateOnly:         true,
		yes:                  true,
		templateMode:         true,
		name:                 projectName,
		prompt:               ui.PromptForValue,
		promptRuntimeOptions: runtimeOptionsMock,
		secretsProvider:      "default",
		templateNameOrURL:    "python",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	require.NoError(t, err)
	proj := loadProject(t, tempdir)
	require.Equal(t, 1, len(proj.Runtime.Options()))
	require.Equal(t, "someValue", proj.Runtime.Options()["someOption"])
}
