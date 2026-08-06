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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

//nolint:paralleltest // changes directory for process
func TestFailInInteractiveWithoutYes(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	args := newArgs{
		interactive:       false,
		yes:               false,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: localTemplate(t),
	}

	err := runNew(t.Context(), args)
	assert.Error(t, err)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestFailIfProjectNameDoesNotMatch(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	mockCurrentBackend(t, &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			parts := strings.Split(s, "/")
			require.Len(t, parts, 3)
			return &backend.MockStackReference{
				ProjectV: tokens.Name(parts[1]),
				NameV:    tokens.MustParseStackName(parts[2]),
			}, nil
		},
	})

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             "org/projectA/stack",
		name:              "projectB",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	assert.ErrorContains(t, err, "project name (--name projectB) "+
		"and stack reference project name (--stack projectA) must be the same")
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedOrgName(t *testing.T) {
	t.Skip("https://github.com/pulumi/pulumi/issues/20410")
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             orgStackName,
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, orgStackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithPromptedOrgName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	uniqueProjectName := filepath.Base(tempdir)
	orgStackName := fmt.Sprintf("%s/%s", currentUser(t), stackName)
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), filepath.Base(tempdir), stackName)

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, orgStackName),
		secretsProvider:   "default",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, orgStackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingStackWithArgsSpecifiedFullNameSucceeds(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	// the project name and the project name in the stack name must match
	uniqueProjectName := filepath.Base(tempdir)
	fullStackName := fmt.Sprintf("%s/%s/%s", currentUser(t), uniqueProjectName, stackName)

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             fullStackName,
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, fullStackName, loadStackName(t))
	removeStack(t, tempdir, fullStackName)
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	uniqueProjectName := filepath.Base(tempdir) + "test"

	args := newArgs{
		interactive:       false,
		yes:               true,
		name:              uniqueProjectName,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	removeStack(t, tempdir, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	uniqueProjectName := filepath.Base(tempdir) + "test"

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(uniqueProjectName, stackName),
		secretsProvider:   "default",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	removeStack(t, tempdir, stackName)

	proj := loadProject(t, tempdir)
	assert.Equal(t, uniqueProjectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestCreatingProjectWithExistingArgsSpecifiedNameFails(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	mockCurrentBackend(t, &backend.MockBackend{
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
		templateNameOrURL: localTemplate(t),
	}

	err := runNew(t.Context(), args)
	assert.ErrorContains(t, err, "project with this name already exists")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestCreatingProjectWithExistingPromptedNameFails(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	mockCurrentBackend(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
		NameF: func() string { return "mock" },
	})

	args := newArgs{
		interactive:       true,
		prompt:            promptMock(projectName, ""),
		secretsProvider:   "default",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	assert.ErrorContains(t, err, "Try again")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGeneratingProjectWithExistingArgsSpecifiedNameSucceeds(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	mockCurrentBackend(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		NameF: func() string { return "mock" },
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	args := newArgs{
		generateOnly:      true,
		interactive:       false,
		yes:               true,
		name:              projectName,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, projectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGeneratingProjectWithExistingPromptedNameSucceeds(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	mockCurrentBackend(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		NameF: func() string { return "mock" },
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	args := newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock(projectName, ""),
		secretsProvider:   "default",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	assert.Equal(t, projectName, proj.Name.String())
}

//nolint:paralleltest // changes directory for process
func TestCreatingProjectWithEmptyConfig(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/4081
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	uniqueProjectName := filepath.Base(tempdir) + "test"

	prompt := func(yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options,
	) (string, error) {
		if strings.Contains(valueType, "(aws:region)") {
			return "", nil
		}
		return defaultValue, nil
	}

	template := writeLocalTemplate(t, t.TempDir(), "aws-template", localTemplateYAML+
		"template:\n  config:\n    aws:region:\n      description: The AWS region to deploy into\n      default: us-east-1\n")

	args := newArgs{
		name:              uniqueProjectName,
		stack:             stackName,
		interactive:       true,
		prompt:            prompt,
		secretsProvider:   "default",
		templateNameOrURL: template,
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(tempdir, "Pulumi."+stackName+".yaml"))
	require.NoError(t, err)

	assert.NotContains(t, projStack.Config, config.MustMakeKey("aws", "region"))

	removeStack(t, tempdir, stackName)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGeneratingProjectWithInvalidArgsSpecifiedNameFails(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	mockCurrentBackend(t, &backend.MockBackend{
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
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	}

	err := runNew(t.Context(), args)
	assert.ErrorContains(t, err, "project names may only contain")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGeneratingProjectWithInvalidPromptedNameFails(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	templateDir := localTemplate(t)

	mockCurrentBackend(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		NameF: func() string { return "mock" },
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	err := runNew(t.Context(), newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock("not#valid", ""),
		secretsProvider:   "default",
		templateNameOrURL: templateDir,
		languageTemplate:  languageTemplateMock,
	})
	assert.ErrorContains(t, err, "project names may only contain")

	err = runNew(t.Context(), newArgs{
		generateOnly:      true,
		interactive:       true,
		prompt:            promptMock("", ""),
		secretsProvider:   "default",
		templateNameOrURL: templateDir,
		languageTemplate:  languageTemplateMock,
	})
	assert.ErrorContains(t, err, "project names may not be empty")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestInvalidTemplateName(t *testing.T) {
	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		t.Chdir(tempdir)

		useLocalTemplateRepo(t, "typescript")
		mockCurrentBackend(t, &backend.MockBackend{
			NameF: func() string { return "mock" },
			GetReadOnlyCloudRegistryF: func() registry.Registry {
				return &backend.MockCloudRegistry{
					Mock: registry.Mock{
						ListTemplatesF: func(
							ctx context.Context, opts registry.ListTemplatesOptions,
						) iter.Seq2[apitype.ListTemplatesResponse, error] {
							return singlePage()
						},
					},
				}
			},
		})

		// A template that will never exist.
		template := "this-is-not-the-template-youre-looking-for"

		args := newArgs{
			interactive:       false,
			yes:               true,
			secretsProvider:   "default",
			templateNameOrURL: template,
		}

		err := runNew(t.Context(), args)
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		t.Chdir(tempdir)

		t.Setenv(env.TemplatePath.Var().Name(), filepath.Join(t.TempDir(), "templates"))

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		args := newArgs{
			generateOnly:      true,
			offline:           true,
			secretsProvider:   "default",
			templateNameOrURL: template,
			yes:               true,
		}

		err := runNew(t.Context(), args)
		assert.ErrorContains(t, err, "not found")
	})
}

func tempProjectDir(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), genUniqueName(t))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	return dir
}

const localTemplateYAML = "name: ${PROJECT}\ndescription: ${DESCRIPTION}\nruntime: yaml\n"

func writeLocalTemplate(t *testing.T, parent, name, body string) string {
	t.Helper()

	dir := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(body), 0o600))
	return dir
}

func localTemplate(t *testing.T) string {
	t.Helper()

	return writeLocalTemplate(t, t.TempDir(), "template", localTemplateYAML)
}

func useLocalTemplateRepo(t *testing.T, names ...string) {
	t.Helper()

	repo := t.TempDir()
	for _, name := range names {
		writeLocalTemplate(t, repo, name, localTemplateYAML)
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	git("init", "--initial-branch=master")
	git("add", ".")
	git("-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
		"commit", "-m", "templates")
	t.Setenv(env.TemplateGitRepository.Var().Name(), repo)
	t.Setenv(env.TemplateBranch.Var().Name(), "master")
	t.Setenv(env.TemplatePath.Var().Name(), filepath.Join(t.TempDir(), "templates"))
}

func mockCurrentBackend(t *testing.T, mockBackend backend.Backend) {
	t.Helper()

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})
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
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						ctx context.Context, opts registry.ListTemplatesOptions,
					) iter.Seq2[apitype.ListTemplatesResponse, error] {
						return singlePage()
					},
				},
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
		t.Run(fmt.Sprintf("project=%q/stackRef=%q", tt.projectName, tt.stackRef), func(t *testing.T) {
			t.Parallel()
			err := compareStackProjectName(b, tt.stackRef, tt.projectName)
			if tt.valid {
				require.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestProjectExists(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
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
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						ctx context.Context, opts registry.ListTemplatesOptions,
					) iter.Seq2[apitype.ListTemplatesResponse, error] {
						return singlePage()
					},
				},
			}
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateProjectName(ctx, b, tt.give.orgName, tt.give.projectName, false /* generateOnly */, display.Options{})
			if tt.hasErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
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
		t.Run(tt.name, func(t *testing.T) {
			tempdir := tempProjectDir(t)
			t.Chdir(tempdir)

			args := newArgs{
				generateOnly:      true,
				interactive:       false,
				yes:               true,
				prompt:            ui.PromptForValue,
				secretsProvider:   "default",
				stack:             tt.stack,
				name:              "project",
				templateNameOrURL: localTemplate(t),
				languageTemplate:  languageTemplateMock,
			}

			err := runNew(t.Context(), args)
			require.NoError(t, err)
		})
	}
}

//nolint:paralleltest // changes directory for process
func TestPulumiNewWithEmptyTemplateSource(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		yes         bool
		wantErr     bool
	}{
		{
			name: "yes creates empty project",
			yes:  true,
		},
		{
			name:        "interactive errors",
			interactive: true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempdir := tempProjectDir(t)
			t.Chdir(tempdir)
			emptyTemplateSource := t.TempDir()

			args := newArgs{
				generateOnly:      true,
				interactive:       tt.interactive,
				yes:               tt.yes,
				prompt:            ui.PromptForValue,
				secretsProvider:   "default",
				templateNameOrURL: emptyTemplateSource,
				languageTemplate:  languageTemplateMock,
			}

			err := runNew(t.Context(), args)
			if tt.wantErr {
				require.ErrorContains(t, err, "no templates")
				_, statErr := os.Stat(filepath.Join(tempdir, "Pulumi.yaml"))
				assert.ErrorIs(t, statErr, os.ErrNotExist)
				return
			}

			require.NoError(t, err)
			proj := loadProject(t, tempdir)
			assert.Equal(t, filepath.Base(tempdir), proj.Name.String())
			assert.Empty(t, proj.Runtime.Name())
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
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						ctx context.Context, opts registry.ListTemplatesOptions,
					) iter.Seq2[apitype.ListTemplatesResponse, error] {
						return singlePage()
					},
				},
			}
		},
	}

	require.NoError(t,
		validateProjectNameInternal(
			t.Context(), b, "moolumi", "some-project-name", false /* generateOnly */, display.Options{},
			func(s string) error {
				assert.Fail(t, "this should not be called as this is a not a duplicate project name")
				return nil
			},
		))

	var called bool
	require.NoError(t,
		validateProjectNameInternal(
			t.Context(), b, "moolumi", "existing-project-name", false /* generateOnly */, display.Options{},
			func(s string) error {
				called = true
				return nil
			},
		))
	assert.Truef(t, called, "expected resolution to be called with duplicate name")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestPulumiNewSetsTemplateTag(t *testing.T) {
	tests := []struct {
		argument string
		// answers drives the guided prompts when no template is named. Empty means the
		// invocation names one and never prompts.
		answers  []any
		expected string
		remote   bool
	}{
		{
			argument: "typescript",
			expected: "typescript",
		},
		{
			argument: "https://github.com/pulumi/templates/tree/master/yaml?foo=bar",
			expected: "https://github.com/pulumi/templates/tree/master/yaml",
			remote:   true,
		},
		{
			answers:  []any{"Basic Pulumi Program", "Python", confirmYes},
			expected: "python",
		},
	}
	for _, tt := range tests {
		name := tt.argument
		if name == "" {
			name = tt.expected
		}
		t.Run(name, func(t *testing.T) {
			if tt.remote {
				skipIfShortOrNoPulumiAccessToken(t)
			} else {
				useLocalTemplateRepo(t, "typescript", "python")
				mockCurrentBackend(t, &backend.MockBackend{
					NameF: func() string { return "mock" },
					GetReadOnlyCloudRegistryF: func() registry.Registry {
						return &backend.MockCloudRegistry{
							Mock: registry.Mock{
								ListTemplatesF: func(
									ctx context.Context, opts registry.ListTemplatesOptions,
								) iter.Seq2[apitype.ListTemplatesResponse, error] {
									return singlePage()
								},
							},
						}
					},
				})
			}

			tempdir := tempProjectDir(t)
			t.Chdir(tempdir)
			uniqueProjectName := filepath.Base(tempdir) + "test"

			prompted := len(tt.answers) > 0
			selectOne := func(string, []string, display.Options) (int, error) {
				t.Error("a named template must not prompt")
				return 0, nil
			}
			if prompted {
				selectOne, _ = scriptedSelect(t, tt.answers...)
			}

			runtimeOptionsMock := func(ctx *plugin.Context, language plugin.LanguageRuntime,
				info *workspace.ProjectRuntimeInfo, main string, opts display.Options, interactive, yes bool,
				prompt promptForValueFunc,
			) (map[string]any, error) {
				return nil, nil
			}

			args := newArgs{
				interactive:          prompted,
				generateOnly:         true,
				yes:                  !prompted,
				name:                 projectName,
				prompt:               promptMock(uniqueProjectName, stackName),
				promptRuntimeOptions: runtimeOptionsMock,
				languageTemplate:     languageTemplateMock,
				selectOne:            selectOne,
				secretsProvider:      "default",
				templateNameOrURL:    tt.argument,
			}

			err := runNew(t.Context(), args)
			require.NoError(t, err)

			proj := loadProject(t, tempdir)
			require.NoError(t, err)
			tagsValue, has := proj.Config[apitype.PulumiTagsConfigKey]
			assert.True(t, has)
			tagsObject, ok := tagsValue.Value.(map[string]any)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, tagsObject[apitype.ProjectTemplateTag])
		})
	}
}

//nolint:paralleltest // changes directory for process
func TestPulumiPromptRuntimeOptions(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	runtimeOptionsMock := func(ctx *plugin.Context, language plugin.LanguageRuntime, info *workspace.ProjectRuntimeInfo,
		main string, opts display.Options, interactive, yes bool, prompt promptForValueFunc,
	) (map[string]any, error) {
		return map[string]any{"someOption": "someValue"}, nil
	}

	template := writeLocalTemplate(t, t.TempDir(), "python-template",
		"name: ${PROJECT}\ndescription: ${DESCRIPTION}\nruntime: python\n")

	args := newArgs{
		interactive:          false,
		generateOnly:         true,
		yes:                  true,
		name:                 projectName,
		prompt:               ui.PromptForValue,
		promptRuntimeOptions: runtimeOptionsMock,
		languageTemplate:     languageTemplateMock,
		secretsProvider:      "default",
		templateNameOrURL:    template,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	require.NoError(t, err)
	proj := loadProject(t, tempdir)
	require.Len(t, proj.Runtime.Options(), 1)
	require.Equal(t, "someValue", proj.Runtime.Options()["someOption"])
}

// runtimeOptionsNone is a promptRuntimeOptions that reports no additional runtime options,
// hoisted so the guided confirmation flow tests below don't each redefine it.
func runtimeOptionsNone(ctx *plugin.Context, language plugin.LanguageRuntime, info *workspace.ProjectRuntimeInfo,
	main string, opts display.Options, yes, interactive bool, prompt promptForValueFunc,
) (map[string]any, error) {
	return nil, nil
}

// countingPrompt answers prompts from a map by valueType, falling back to the default,
// and records every prompt it is asked.
func countingPrompt(answers map[string]string, log *[]string) promptForValueFunc {
	return func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		*log = append(*log, valueType+"="+defaultValue)
		value, ok := answers[valueType]
		if !ok {
			value = defaultValue
		}
		if isValidFn != nil {
			if err := isValidFn(value); err != nil {
				return "", err
			}
		}
		return value, nil
	}
}

// guidedRepoTemplate points the template source at a local one-template repo whose Pulumi.yaml is
// the given body, mirroring useLocalTemplateRepo but for a single template with a custom body (the
// guided confirmation tests need a template that declares its own config block).
func guidedRepoTemplate(t *testing.T, name, body string) {
	t.Helper()

	repo := t.TempDir()
	writeLocalTemplate(t, repo, name, body)
	git := func(cliArgs ...string) {
		cmd := exec.Command("git", cliArgs...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", cliArgs, out)
	}
	git("init", "--initial-branch=master")
	git("add", ".")
	git("-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
		"commit", "-m", "init")
	t.Setenv(env.TemplateGitRepository.Var().Name(), repo)
	t.Setenv(env.TemplateBranch.Var().Name(), "master")
	t.Setenv(env.TemplatePath.Var().Name(), filepath.Join(t.TempDir(), "templates"))
}

// guidedConfigTemplateYAML is the template body shared by the guided confirmation flow tests: a
// runtime with no install-time dependencies (so the full, non-generate-only flow stays offline),
// and one config key with a default so the "accept" path never has to prompt for it.
const guidedConfigTemplateYAML = `name: ${PROJECT}
description: ${DESCRIPTION}
runtime: yaml
template:
  description: A guided test template
  config:
    aws:region:
      description: The AWS region to deploy into
      default: us-east-1
`

// guidedNewTestBackend builds the mock backend the guided confirmation flow tests share: an
// org-backed backend whose stack creation, secrets manager, and config-save calls all resolve
// locally so the whole runNew flow runs offline. created records every stack name CreateStack saw.
func guidedNewTestBackend(t *testing.T) (*backend.MockBackend, *[]string) {
	t.Helper()

	created := &[]string{}
	b := stackCreationBackend(t, 0, created)
	b.GetDefaultOrgF = func(ctx context.Context) (string, error) { return "my-org", nil }
	b.SupportsOrganizationsF = func() bool { return true }
	b.DoesProjectExistF = func(ctx context.Context, org, name string) (bool, error) { return false, nil }
	b.NameF = func() string { return "mock" }
	b.URLF = func() string { return "mock://guided" }
	b.SetCurrentProjectF = func(proj *workspace.Project) {}
	b.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
		return nil, nil
	}
	b.GetLatestConfigurationF = func(ctx context.Context, s backend.Stack) (backend.LatestConfiguration, error) {
		return backend.LatestConfiguration{}, backenderr.ErrNoPreviousDeployment
	}
	b.ParseStackReferenceF = func(s string) (backend.StackReference, error) {
		parts := strings.Split(s, "/")
		return &backend.MockStackReference{
			NameV:               tokens.MustParseStackName(parts[len(parts)-1]),
			StringV:             s,
			FullyQualifiedNameV: tokens.QName(s),
		}, nil
	}
	mockSecretsManager := func(ctx context.Context, ps *workspace.ProjectStack) (secrets.Manager, error) {
		return &secrets.MockSecretsManager{
			TypeF:  func() string { return "mock" },
			StateF: func() json.RawMessage { return nil },
			EncrypterF: func() config.Encrypter {
				return &secrets.MockEncrypter{EncryptValueF: func(s string) string { return s }}
			},
			DecrypterF: func() config.Decrypter {
				return &secrets.MockDecrypter{DecryptValueF: func(s string) string { return s }}
			},
		}, nil
	}
	b.CreateStackF = func(ctx context.Context, ref backend.StackReference, root string,
		initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
	) (backend.Stack, error) {
		*created = append(*created, ref.String())
		return &backend.MockStack{
			RefF:                  func() backend.StackReference { return ref },
			BackendF:              func() backend.Backend { return b },
			SnapshotF:             func(ctx context.Context, sp secrets.Provider) (*deploy.Snapshot, error) { return nil, nil },
			DefaultSecretManagerF: mockSecretsManager,
		}, nil
	}
	b.GetReadOnlyCloudRegistryF = func() registry.Registry {
		return &backend.MockCloudRegistry{Mock: registry.Mock{
			ListTemplatesF: func(ctx context.Context, opts registry.ListTemplatesOptions,
			) iter.Seq2[apitype.ListTemplatesResponse, error] {
				return singlePage()
			},
		}}
	}
	return b, created
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewAcceptSkipsAllPrompts(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	var prompts []string
	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmYes)
	var out bytes.Buffer
	args := newArgs{
		interactive: true,
		// aws:profile isn't declared by the template's config block at all: it must still
		// reach the saved stack config, alongside the declared aws:region.
		configArray:          []string{"aws:profile=work"},
		prompt:               countingPrompt(nil, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Empty(t, prompts, "accepting the block must not prompt for anything")
	require.Equal(t, []string{"my-org/dev"}, *created)
	assert.Contains(t, out.String(), "Stack name:    my-org/dev")
	assert.Contains(t, out.String(), "aws:region:  us-east-1")
	assert.NotContains(t, out.String(), "Created project")
	assert.Contains(t, out.String(), "Project:  "+filepath.Base(tempdir))
	assert.Contains(t, out.String(), "Stack:    my-org/dev")
	proj := loadProject(t, tempdir)
	assert.Equal(t, "A guided test template", *proj.Description)

	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(tempdir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	region, ok := projStack.Config[config.MustMakeKey("aws", "region")]
	require.True(t, ok, "the declared key with a default must still be saved")
	regionValue, err := region.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", regionValue)
	profile, ok := projStack.Config[config.MustMakeKey("aws", "profile")]
	require.True(t, ok, "a --config key the template doesn't declare must not be dropped")
	profileValue, err := profile.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "work", profileValue)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewDeclineRepromptsPrefilled(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmChange)
	var prompts []string
	var out bytes.Buffer
	args := newArgs{
		interactive:          true,
		prompt:               countingPrompt(map[string]string{"Stack name": "prod"}, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"Project name=" + filepath.Base(tempdir),
		"Project description=A guided test template",
		"Stack name=my-org/dev",
		"The AWS region to deploy into (aws:region)=us-east-1",
	}, prompts, "every reprompt must be pre-filled with the value the block just showed")
	assert.Equal(t, []string{"my-org/prod"}, *created, "a bare typed name still org-resolves")

	proj := loadProject(t, tempdir)
	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(tempdir, "Pulumi.prod.yaml"))
	require.NoError(t, err)
	region, ok := projStack.Config[config.MustMakeKey("aws", "region")]
	require.True(t, ok, "the pre-filled config value must still land once re-confirmed")
	regionValue, err := region.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", regionValue)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewDeclineDoesNotReofferFlagSettledConfig(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmChange)
	var prompts []string
	var out bytes.Buffer
	args := newArgs{
		interactive: true,
		// aws:region has a template default of us-east-1, but a --config value wins:
		// it must show in the block and never be re-offered on decline.
		configArray:          []string{"aws:region=eu-west-1"},
		prompt:               countingPrompt(nil, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "aws:region:  eu-west-1", "the block must show the flag value")
	assert.Equal(t, []string{
		"Project name=" + filepath.Base(tempdir),
		"Project description=A guided test template",
		"Stack name=my-org/dev",
	}, prompts, "a flag-settled key must never be re-prompted, on decline or otherwise")
	require.Equal(t, []string{"my-org/dev"}, *created)

	proj := loadProject(t, tempdir)
	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(tempdir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	region, ok := projStack.Config[config.MustMakeKey("aws", "region")]
	require.True(t, ok)
	regionValue, err := region.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", regionValue)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewNoDefaultConfigAskedBeforeBlock(t *testing.T) {
	const noDefaultConfigTemplateYAML = `name: ${PROJECT}
description: ${DESCRIPTION}
runtime: yaml
template:
  description: A guided GCP test template
  config:
    gcp:project:
      description: The Google Cloud project to deploy into
`
	guidedRepoTemplate(t, "gcp-python", noDefaultConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "Google Cloud", "Python", confirmYes)
	var prompts []string
	var out bytes.Buffer
	args := newArgs{
		interactive: true,
		prompt: countingPrompt(map[string]string{
			"The Google Cloud project to deploy into (gcp:project)": "proj-123",
		}, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, []string{"The Google Cloud project to deploy into (gcp:project)="}, prompts,
		"a key with no default must be prompted for with an empty default, before the block")
	assert.Contains(t, out.String(), "gcp:project:  proj-123")
	require.Equal(t, []string{"my-org/dev"}, *created)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewCollidingDefaultNameFallsThrough(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	defaultName := filepath.Base(tempdir)

	b, created := guidedNewTestBackend(t)
	b.DoesProjectExistF = func(ctx context.Context, org, name string) (bool, error) {
		return name == defaultName, nil
	}
	mockCurrentBackend(t, b)

	// selects: "AWS", "Python" — and nothing else: scriptedSelect fails the test on an
	// unexpected confirmation select, proving the flow fell through to sequential prompts.
	selectOne, _ := scriptedSelect(t, "AWS", "Python")
	var prompts []string
	var out bytes.Buffer
	args := newArgs{
		interactive:          true,
		prompt:               countingPrompt(map[string]string{"Project name": "fresh-name"}, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	require.NotEmpty(t, prompts)
	assert.Equal(t, "Project name="+defaultName, prompts[0],
		"the sequential prompts must run, starting with the project name")
	assert.NotContains(t, out.String(), "Project name:  ")
	require.Equal(t, []string{"my-org/dev"}, *created)
}

//nolint:paralleltest // Sets a mock login manager
func TestPulumiNewWithOrgTemplates(t *testing.T) {
	// Set environment variable to disable registry resolution and use org templates
	t.Setenv("PULUMI_DISABLE_REGISTRY_RESOLVE", "true")
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
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						ctx context.Context, opts registry.ListTemplatesOptions,
					) iter.Seq2[apitype.ListTemplatesResponse, error] {
						return singlePage()
					},
				},
			}
		},
	}
	mockCurrentBackend(t, mockBackend)

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
  aws-python                         A minimal AWS Python Pulumi program
  aws-scala                          A minimal AWS Scala Pulumi program
  aws-typescript                     A minimal AWS TypeScript Pulumi program
  aws-visualbasic                    A minimal AWS VB.NET Pulumi program
  aws-yaml                           A minimal AWS Pulumi YAML program
`)
	assert.Equal(t, "", stderr.String())
}

func ptr[T any](v T) *T { return &v }

// singlePage answers a template listing with one page holding the given templates.
func singlePage(templates ...apitype.TemplateMetadata) iter.Seq2[apitype.ListTemplatesResponse, error] {
	return func(yield func(apitype.ListTemplatesResponse, error) bool) {
		yield(apitype.ListTemplatesResponse{Templates: templates}, nil)
	}
}

//nolint:paralleltest // Sets a mock login manager
func TestPulumiNewWithRegistryTemplates(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_REGISTRY_RESOLVE", "false")
	t.Setenv("PULUMI_EXPERIMENTAL", "true")
	mockRegistry := &backend.MockCloudRegistry{
		Mock: registry.Mock{
			ListTemplatesF: func(
				ctx context.Context, opts registry.ListTemplatesOptions,
			) iter.Seq2[apitype.ListTemplatesResponse, error] {
				return singlePage(apitype.TemplateMetadata{
					Name: "template-1", Description: ptr("Describe 1"), Publisher: "Some org",
				}, apitype.TemplateMetadata{
					Name: "template-2", Description: ptr("Describe 2"), RepoSlug: ptr("some-org/repo"), Source: "github",
				})
			},
		},
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return mockRegistry
		},
	}
	mockCurrentBackend(t, mockBackend)

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
	// Check that our registry based templates are there with the appropriate disambiguation prefix.
	assert.Contains(t, stdout.String(), "template-1 [Some org]              Describe 1")
	assert.Contains(t, stdout.String(), "template-2 [some-org/repo]         Describe 2")

	// Check that normal templates are there
	assertTemplateContains(t, stdout.String(), `
  aws-csharp                         A minimal AWS C# Pulumi program
  aws-fsharp                         A minimal AWS F# Pulumi program
  aws-go                             A minimal AWS Go Pulumi program
  aws-java                           A minimal AWS Java Pulumi program
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
func TestPulumiNewWithoutPulumiAccessToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping template registry integration test in short mode")
	}

	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	tempdir := t.TempDir()
	t.Setenv("PULUMI_HOME", tempdir)
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
  aws-python                              A minimal AWS Python Pulumi program
  aws-scala                               A minimal AWS Scala Pulumi program
  aws-typescript                          A minimal AWS TypeScript Pulumi program
  aws-visualbasic                         A minimal AWS VB.NET Pulumi program
  aws-yaml                                A minimal AWS Pulumi YAML program
`)
	assert.Equal(t, "", stderr.String())
}

//nolint:paralleltest // Sets a mock login manager
func TestPulumiNewWithoutTemplateSupport(t *testing.T) {
	mockCurrentBackend(t, &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						ctx context.Context, opts registry.ListTemplatesOptions,
					) iter.Seq2[apitype.ListTemplatesResponse, error] {
						require.Len(t, opts.Backing, 1, "browsing splits the listing, one backing per fetch")
						return singlePage()
					},
				},
			}
		},
		NameF: func() string { return "mock" },
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

//nolint:paralleltest // Sets a mock login manager, changes the directory
func TestPulumiNewOrgTemplate(t *testing.T) {
	// Set environment variable to disable registry resolution and use org templates
	t.Setenv("PULUMI_DISABLE_REGISTRY_RESOLVE", "true")
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
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
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						ctx context.Context, opts registry.ListTemplatesOptions,
					) iter.Seq2[apitype.ListTemplatesResponse, error] {
						return singlePage()
					},
				},
			}
		},
	}
	mockCurrentBackend(t, mockBackend)

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

func languageTemplateMock(ctx context.Context, language plugin.LanguageRuntime, info plugin.ProgramInfo,
	projectName tokens.PackageName,
) error {
	return nil
}

// =======================
// Tests for pulumi new -y
// =======================

// useTempFilestateBackend points the backend at a temp directory so tests don't hit the real backend.
func useTempFilestateBackend(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "file://"+filepath.ToSlash(t.TempDir()))
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
}

//nolint:paralleltest
func TestNewCmdYesWritesMinimalPulumiYAMLWithExplicitName(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := NewNewCmd()
	cmd.SetArgs([]string{"-y", "--name", "my-project"})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: my-project\n", string(contents))
}

//nolint:paralleltest
func TestNewCmdYesUsesCurrentDirectoryNameByDefault(t *testing.T) {
	useTempFilestateBackend(t)
	dir := filepath.Join(t.TempDir(), "my-project")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	cmd := NewNewCmd()
	cmd.SetArgs([]string{"-y"})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: my-project\n", string(contents))
}

//nolint:paralleltest
func TestNewCmdYesSanitizesDefaultDirectoryName(t *testing.T) {
	useTempFilestateBackend(t)
	dir := filepath.Join(t.TempDir(), "my project!")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	cmd := NewNewCmd()
	cmd.SetArgs([]string{"-y"})

	err := cmd.Execute()

	require.NoError(t, err)
	contents, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "name: myproject\n", string(contents))
}

//nolint:paralleltest
func TestNewCmdYesRejectsInvalidExplicitName(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := NewNewCmd()
	cmd.SetArgs([]string{"-y", "--name", "my project"})

	err := cmd.Execute()

	require.ErrorContains(t, err, "'my project' is not a valid project name")
	_, statErr := os.Stat(filepath.Join(dir, "Pulumi.yaml"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// requiredPackagesRecorder is an in-process language runtime, attached via
// PULUMI_DEBUG_LANGUAGES, that records the GetRequiredPackages call it receives and
// reports a single required package.
type requiredPackagesRecorder struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	programDirectory atomic.Value
	requiredPackage  *pulumirpc.PackageDependency
}

func (s *requiredPackagesRecorder) Handshake(
	context.Context, *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (s *requiredPackagesRecorder) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

func (s *requiredPackagesRecorder) InstallDependencies(
	*pulumirpc.InstallDependenciesRequest, pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (s *requiredPackagesRecorder) GetRequiredPackages(
	_ context.Context, req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	s.programDirectory.Store(req.Info.ProgramDirectory)
	return &pulumirpc.GetRequiredPackagesResponse{
		Packages: []*pulumirpc.PackageDependency{s.requiredPackage},
	}, nil
}

func pluginBinaryName(name string) string {
	binary := "pulumi-resource-" + name
	if goruntime.GOOS == "windows" {
		binary += ".exe"
	}
	return binary
}

// fakePluginServer serves a plugin tarball containing a stub pulumi-resource-<name>
// binary, in the layout the plugin download machinery expects.
func fakePluginServer(t *testing.T, name string) *httptest.Server {
	t.Helper()

	var tarball bytes.Buffer
	gzw := gzip.NewWriter(&tarball)
	tw := tar.NewWriter(gzw)
	binary := []byte("#!/bin/sh\nexit 0\n")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: pluginBinaryName(name),
		Mode: 0o755,
		Size: int64(len(binary)),
	}))
	_, err := tw.Write(binary)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(tarball.Bytes())
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	return server
}

// TestNewInstallsRequiredPackages ensures that `pulumi new` resolves & installs required packages.
func TestNewInstallsRequiredPackages(t *testing.T) {
	useTempFilestateBackend(t)
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)

	pluginServer := fakePluginServer(t, "testpkg")
	lang := &requiredPackagesRecorder{
		requiredPackage: &pulumirpc.PackageDependency{
			Name:    "testpkg",
			Kind:    "resource",
			Version: "1.2.3",
			Server:  pluginServer.URL,
		},
	}
	cancel := make(chan bool)
	t.Cleanup(func() { close(cancel) })
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, lang)
			return nil
		},
	})
	require.NoError(t, err)
	t.Setenv("PULUMI_DEBUG_LANGUAGES", fmt.Sprintf("testlang:%d", handle.Port))

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	template := writeLocalTemplate(t, t.TempDir(), "testlang-template",
		"name: ${PROJECT}\ndescription: ${DESCRIPTION}\nruntime: testlang\n")

	args := newArgs{
		interactive:       false,
		yes:               true,
		prompt:            ui.PromptForValue,
		secretsProvider:   "default",
		stack:             stackName,
		templateNameOrURL: template,
		languageTemplate:  languageTemplateMock,
	}
	require.NoError(t, runNew(t.Context(), args))

	assert.Equal(t, tempdir, lang.programDirectory.Load())
	assert.FileExists(t,
		filepath.Join(pulumiHome, "plugins", "resource-testpkg-v1.2.3", pluginBinaryName("testpkg")))
}

//nolint:paralleltest
func TestNewCmdYesDoesNotOverwriteExistingPulumiYAML(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	existing := filepath.Join(dir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(existing, []byte("name: existing\n"), 0o600))
	t.Chdir(dir)
	cmd := NewNewCmd()
	cmd.SetArgs([]string{"-y"})

	err := cmd.Execute()

	require.ErrorContains(t, err, dir+" is not empty;")
	contents, readErr := os.ReadFile(existing)
	require.NoError(t, readErr)
	assert.Equal(t, "name: existing\n", string(contents))
}
