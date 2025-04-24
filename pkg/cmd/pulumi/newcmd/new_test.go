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
	"bytes"
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
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)

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

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedOrgName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	uniqueProjectName := filepath.Base(tempdir)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, orgStackName),
		secretsProvider:   "default",
		templateNameOrURL: "typescript",
	}

	err := runNew(context.Background(), args)
	assert.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedFullNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	// the project name and the project name in the stack name must match
	uniqueProjectName := filepath.Base(tempdir)
	owner := currentUser(t)
	fullStackName := fmt.Sprintf("%s/%s/%s", owner, uniqueProjectName, stackName)

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

	assert.Equal(t, fullStackName, loadStackName(t))
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

	testutil.MockBackendInstance(t, &backend.MockBackend{
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

	testutil.MockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
		SupportsTemplatesF: func() bool { return false },
		NameF:              func() string { return "mock" },
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

	testutil.MockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		SupportsTemplatesF: func() bool { return false },
		NameF:              func() string { return "mock" },
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

	testutil.MockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		SupportsTemplatesF: func() bool { return false },
		NameF:              func() string { return "mock" },
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

	testutil.MockBackendInstance(t, &backend.MockBackend{
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

	testutil.MockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		SupportsTemplatesF: func() bool { return false },
		NameF:              func() string { return "mock" },
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

			chooseTemplateMock := func(templates []cmdTemplates.Template, opts display.Options,
			) (cmdTemplates.Template, error) {
				for _, template := range templates {
					if template.Name() == tt.prompted {
						return template, nil
					}
				}
				return nil, errors.New("template not found")
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

//nolint:paralleltest // Sets a global mock backend
func TestPulumiNewWithOrgTemplates(t *testing.T) {
	mockBackend := &backend.MockBackend{
		SupportsTemplatesF: func() bool { return true },
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "fred", []string{"org1", "personal"}, nil, nil
		},
		ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
			switch orgName {
			case "org1":
				return apitype.ListOrgTemplatesResponse{
					OrgHasTemplates: true,
					Templates: map[string][]*apitype.PulumiTemplateRemote{
						"github.com/example/foo": {
							{
								SourceName:  "Foo",
								Name:        "template-1",
								TemplateURL: "github.com/example/foo/template-1",
								ProjectTemplate: apitype.ProjectTemplate{
									DisplayName: "Display 1",
									Description: "Describe 1",
								},
							},
							{
								SourceName:  "Foo",
								Name:        "template-2",
								TemplateURL: "github.com/example/foo/template-2",
								ProjectTemplate: apitype.ProjectTemplate{
									DisplayName: "Display 2",
									Description: "Describe 2",
								},
							},
						},
					},
				}, nil
			case "personal":
				return apitype.ListOrgTemplatesResponse{OrgHasTemplates: false}, nil
			default:
				return apitype.ListOrgTemplatesResponse{}, fmt.Errorf("unknown org %q", orgName)
			}
		},
	}
	testutil.MockBackendInstance(t, mockBackend)
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	newCmd := NewNewCmd()
	var stdout, stderr bytes.Buffer
	newCmd.SetOut(&stdout)
	newCmd.SetErr(&stderr)
	newCmd.SetArgs([]string{"--list-templates"})
	err := newCmd.Execute()
	require.NoError(t, err)

	// Check that the normal prefix is still there
	assert.Contains(t, stdout.String(), `
Available Templates:
`)
	// Check that our org based templates are there
	assert.Contains(t, stdout.String(), `
  template-1                         Describe 1
  template-2                         Describe 2
`)

	// Check that normal templates are there
	assertTemplateContains(t, stdout.String(), `
  aws-csharp                         A minimal AWS C# Pulumi program
  aws-fsharp                         A minimal AWS F# Pulumi program
  aws-go                             A minimal AWS Go Pulumi program
  aws-java                           A minimal AWS Java Pulumi program
  aws-javascript                     A minimal AWS JavaScript Pulumi program
  aws-python                         A minimal AWS Python Pulumi program
  aws-scala                          A minimal AWS Scala Pulumi program
  aws-typescript                     A minimal AWS TypeScript Pulumi program
  aws-visualbasic                    A minimal AWS VB.NET Pulumi program
  aws-yaml                           A minimal AWS Pulumi YAML program
`)
	assert.Equal(t, "", stderr.String())
}

// TestPulumiNewWithoutPulumiAccessToken checks that we won't error if we run `pulumi new
// --list-templates` without PULUMI_ACCESS_TOKEN set.
//
//nolint:paralleltest // Changes environmental variables
func TestPulumiNewWithoutPulumiAccessToken(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	newCmd := NewNewCmd()
	var stdout, stderr bytes.Buffer
	newCmd.SetOut(&stdout)
	newCmd.SetErr(&stderr)
	newCmd.SetArgs([]string{"--list-templates"})
	err := newCmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), `
Available Templates:
`)
	assertTemplateContains(t, stdout.String(), `
  aws-csharp                              A minimal AWS C# Pulumi program
  aws-fsharp                              A minimal AWS F# Pulumi program
  aws-go                                  A minimal AWS Go Pulumi program
  aws-java                                A minimal AWS Java Pulumi program
  aws-javascript                          A minimal AWS JavaScript Pulumi program
  aws-python                              A minimal AWS Python Pulumi program
  aws-scala                               A minimal AWS Scala Pulumi program
  aws-typescript                          A minimal AWS TypeScript Pulumi program
  aws-visualbasic                         A minimal AWS VB.NET Pulumi program
  aws-yaml                                A minimal AWS Pulumi YAML program
`)
	assert.Equal(t, "", stderr.String())
}

//nolint:paralleltest // Sets a global mock backend
func TestPulumiNewWithoutTemplateSupport(t *testing.T) {
	testutil.MockBackendInstance(t, &backend.MockBackend{
		SupportsTemplatesF: func() bool { return false },
		NameF:              func() string { return "mock" },
	})

	newCmd := NewNewCmd()
	var stdout, stderr bytes.Buffer
	newCmd.SetOut(&stdout)
	newCmd.SetErr(&stderr)
	newCmd.SetArgs([]string{"--list-templates"})
	err := newCmd.Execute()
	require.NoError(t, err)

	// Check that normal templates are there
	assert.Contains(t, stdout.String(), `
Available Templates:
  aiven-go                           A minimal Aiven Go Pulumi program
`)
	assert.Equal(t, "", stderr.String())
}

// We should be able to list the templates even when not logged in.
// Regression test for https://github.com/pulumi/pulumi/issues/19073
//
//nolint:paralleltest // Modifies env
func TestPulumiNewNotLoggedIn(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	newCmd := NewNewCmd()
	var stdout, stderr bytes.Buffer
	newCmd.SetOut(&stdout)
	newCmd.SetErr(&stderr)
	newCmd.SetArgs([]string{"--list-templates"})
	err := newCmd.Execute()
	require.NoError(t, err)

	// Check that normal templates are there
	assert.Contains(t, stdout.String(), `
Available Templates:
  aiven-go                           A minimal Aiven Go Pulumi program
`)
	assert.Equal(t, "", stderr.String())
}

//nolint:paralleltest // Sets a global mock backend, changes the directory
func TestPulumiNewOrgTemplate(t *testing.T) {
	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	mockBackend := &backend.MockBackend{
		SupportsTemplatesF: func() bool { return true },
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "fred", []string{"org1"}, nil, nil
		},
		ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
			switch orgName {
			case "org1":
				return apitype.ListOrgTemplatesResponse{
					OrgHasTemplates: true,
					Templates: map[string][]*apitype.PulumiTemplateRemote{
						"github.com/example/foo": {
							{
								SourceName:  "Foo",
								Name:        "template-1",
								TemplateURL: "https://github.com/example/foo/template-1",
								ProjectTemplate: apitype.ProjectTemplate{
									DisplayName: "Display 1",
									Description: "Describe 1",
								},
							},
						},
					},
				}, nil
			default:
				return apitype.ListOrgTemplatesResponse{}, fmt.Errorf("unknown org %q", orgName)
			}
		},
		DownloadTemplateF: func(_ context.Context, orgName, templateSource string) (backend.TarReaderCloser, error) {
			if orgName != "org1" {
				return nil, fmt.Errorf("unknown org %q", orgName)
			}
			if templateSource != "https://github.com/example/foo/template-1" {
				return nil, fmt.Errorf("unknown template source %q", templateSource)
			}

			return backend.MockTarReader{
				"Pulumi.yaml": {Content: `name: ${PROJECT}
description: ${DESCRIPTION}
runtime: yaml
template:
  description: Describe 1

resources:
  # Create an AWS resource (S3 Bucket)
  my-bucket:
    type: aws:s3:BucketV2
`},
			}, nil
		},
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	newCmd := NewNewCmd()
	var stdout, stderr bytes.Buffer
	newCmd.SetOut(&stdout)
	newCmd.SetErr(&stderr)
	newCmd.SetArgs([]string{"template-1", "--generate-only", "--yes"})
	err := newCmd.Execute()
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	require.Equal(t, "yaml", proj.Runtime.Name())
}

// Assert that actual contains the template rows show in expected.
//
// This parsing based comparison is necessary since raw string comparison is unstable
// under insertion due to white-space changes.
func assertTemplateContains(t *testing.T, actual, expected string) {
	parse := func(stdout string) []struct{ name, description string } {
		stdout = strings.TrimPrefix(stdout, `Available Templates:
`)
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		out := make([]struct{ name, description string }, len(lines))
		for i, l := range lines {
			parts := strings.Fields(l)
			out[i].name = parts[0]
			out[i].description = strings.Join(parts[1:], " ")
		}
		return out
	}

	expectedP := parse(expected)
	actualP := parse(actual)
	for _, e := range expectedP {
		assert.Contains(t, actualP, e)
	}
}
