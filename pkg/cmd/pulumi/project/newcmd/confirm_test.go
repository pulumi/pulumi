// Copyright 2026, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestAskConfirmationRendersBlock(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	opts := display.Options{Color: colors.Never, Stdout: &out}
	sel, offered := scriptedSelect(t, confirmYes)

	ok, err := askConfirmation(&out, opts, sel, []field{
		{"Project name", "my-project"},
		{"Description", "A minimal AWS TypeScript Pulumi program"},
		{"Stack name", "my-org/dev"},
	}, []field{{"aws:region", "us-east-1"}})
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t,
		"\n"+
			"Project name:  my-project\n"+
			"Description:   A minimal AWS TypeScript Pulumi program\n"+
			"Stack name:    my-org/dev\n"+
			"Config:\n"+
			"  aws:region:  us-east-1\n",
		out.String())
	require.Len(t, *offered, 1)
	assert.Equal(t, []string{confirmYes, confirmChange}, (*offered)[0])
}

func TestAskConfirmationOmitsEmptyConfig(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	opts := display.Options{Color: colors.Never, Stdout: &out}
	sel, _ := scriptedSelect(t, confirmChange)

	ok, err := askConfirmation(&out, opts, sel, []field{{"Project name", "p"}}, nil)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotContains(t, out.String(), "Config:")
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
func guidedNewTestBackend(t *testing.T) (b *backend.MockBackend, created *[]string) {
	t.Helper()

	created = &[]string{}
	b = stackCreationBackend(t, 0, created)
	b.GetDefaultOrgF = func(ctx context.Context) (string, error) { return "my-org", nil }
	b.SupportsOrganizationsF = func() bool { return true }
	b.DoesProjectExistF = func(ctx context.Context, org, name string) (bool, error) { return false, nil }
	b.NameF = func() string { return "mock" }
	b.URLF = func() string { return "mock://guided" }
	b.SetCurrentProjectF = func(proj *workspace.Project) {}
	b.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
		return nil, nil
	}
	b.ListStackNamesF = func(ctx context.Context, filter backend.ListStackNamesFilter,
		token backend.ContinuationToken,
	) ([]backend.StackReference, backend.ContinuationToken, error) {
		return nil, nil, nil
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
				// A non-identity transform: tests that read the saved YAML back can tell a
				// secure value apart from its plaintext (unlike an identity "encrypter", the
				// ciphertext never contains the plaintext as a substring).
				return &secrets.MockEncrypter{EncryptValueF: func(s string) string {
					return base64.StdEncoding.EncodeToString([]byte(s))
				}}
			},
			DecrypterF: func() config.Decrypter {
				return &secrets.MockDecrypter{DecryptValueF: func(s string) string {
					plain, err := base64.StdEncoding.DecodeString(s)
					if err != nil {
						return s
					}
					return string(plain)
				}}
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
	var out, errOut bytes.Buffer
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
		stderr:               &errOut,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Empty(t, prompts, "accepting the block must not prompt for anything")
	require.Equal(t, []string{"my-org/dev"}, *created)
	assert.NotContains(t, errOut.String(), "Created stack", "guided stack creation must run quiet")
	assert.Contains(t, out.String(), "Stack name:    my-org/dev")
	assert.Contains(t, out.String(), "aws:region:  us-east-1")
	assert.NotContains(t, out.String(), "Created project")
	assert.Contains(t, out.String(), "Project name:  "+filepath.Base(tempdir))
	assert.Contains(t, out.String(), "Stack name:    my-org/dev")
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
func TestGuidedNewConfirmationInterruptIsFriendly(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	// selects: "AWS", "Python" — then Ctrl-C at the confirmation itself.
	selectOne, _ := scriptedSelect(t, "AWS", "Python", terminal.InterruptErr)
	var out bytes.Buffer
	args := newArgs{
		interactive:          true,
		prompt:               ui.PromptForValue,
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
	}

	err := runNew(t.Context(), args)

	require.Error(t, err)
	assert.ErrorIs(t, err, errConfirmationInterrupted)
	assert.Empty(t, *created, "Ctrl-C at the confirmation must not create a stack")
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
	var out, errOut bytes.Buffer
	args := newArgs{
		interactive:          true,
		prompt:               countingPrompt(map[string]string{"Stack name": "prod"}, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
		stderr:               &errOut,
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
	assert.NotContains(t, errOut.String(), "Created stack", "guided stack creation must run quiet even after decline")

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
func TestGuidedNewDeclineDoesNotReofferFlagConfig(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmChange)
	var prompts []string
	var out, errOut bytes.Buffer
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
		stderr:               &errOut,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Contains(t, out.String(), "aws:region:  eu-west-1", "the block must show the flag value")
	assert.Equal(t, []string{
		"Project name=" + filepath.Base(tempdir),
		"Project description=A guided test template",
		"Stack name=my-org/dev",
	}, prompts, "a key fixed by --config must never be re-prompted, on decline or otherwise")
	require.Equal(t, []string{"my-org/dev"}, *created)
	assert.NotContains(t, errOut.String(), "Created stack", "guided stack creation must run quiet")

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
	var out, errOut bytes.Buffer
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
		stderr:               &errOut,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, []string{"The Google Cloud project to deploy into (gcp:project)="}, prompts,
		"a key with no default must be prompted for with an empty default, before the block")
	assert.Contains(t, out.String(), "gcp:project:  proj-123")
	require.Equal(t, []string{"my-org/dev"}, *created)
	assert.NotContains(t, errOut.String(), "Created stack", "guided stack creation must run quiet")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewSecretConfigIsMaskedAndEncrypted(t *testing.T) {
	const secretConfigTemplateYAML = `name: ${PROJECT}
description: ${DESCRIPTION}
runtime: yaml
template:
  description: A guided secret test template
  config:
    github:token:
      description: The GitHub token to use
      secret: true
`
	guidedRepoTemplate(t, "aws-python", secretConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmYes)
	var secretFlags []bool
	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		secretFlags = append(secretFlags, secret)
		if valueType == "The GitHub token to use (github:token)" {
			return "tok_secret_value", nil
		}
		return defaultValue, nil
	}
	var out, errOut bytes.Buffer
	args := newArgs{
		interactive:          true,
		prompt:               prompt,
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
		stderr:               &errOut,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	require.Len(t, secretFlags, 1, "only the key with no default is prompted for, before the block")
	assert.True(t, secretFlags[0], "the pre-block prompt for a secret key must be marked secret")
	assert.Contains(t, out.String(), "github:token:  [secret]", "the block must mask the secret value, not show it")
	assert.NotContains(t, out.String(), "tok_secret_value", "the plaintext secret must never reach the block")
	require.Equal(t, []string{"my-org/dev"}, *created)
	assert.NotContains(t, errOut.String(), "Created stack", "guided stack creation must run quiet")

	rawYAML, err := os.ReadFile(filepath.Join(tempdir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(rawYAML), "tok_secret_value",
		"the saved stack config must never store the secret in plaintext")
	encrypted := base64.StdEncoding.EncodeToString([]byte("tok_secret_value"))
	assert.Contains(t, string(rawYAML), "secure: "+encrypted,
		"the saved stack config must mark the value secure (encrypted via the test secrets manager)")
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewCollidingDefaultNameAsksThenConfirms(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	defaultName := filepath.Base(tempdir)

	b, created := guidedNewTestBackend(t)
	b.DoesProjectExistF = func(ctx context.Context, org, name string) (bool, error) {
		return name == defaultName, nil
	}
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmYes)
	var prompts []string
	var out, errOut bytes.Buffer
	args := newArgs{
		interactive:          true,
		prompt:               countingPrompt(map[string]string{"Project name": "fresh-name"}, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
		stderr:               &errOut,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, []string{"Project name=" + defaultName}, prompts,
		"an unusable default name is the only thing asked before the block")
	assert.Contains(t, out.String(), "Project name:  fresh-name",
		"the block confirms the name that was just chosen")
	assert.Contains(t, out.String(), "Stack name:    my-org/dev")
	require.Equal(t, []string{"my-org/dev"}, *created)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewExistingStackNameAsksBeforeBlock(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	var listedProjects []string
	b.ListStackNamesF = func(ctx context.Context, filter backend.ListStackNamesFilter,
		token backend.ContinuationToken,
	) ([]backend.StackReference, backend.ContinuationToken, error) {
		require.NotNil(t, filter.Project)
		listedProjects = append(listedProjects, *filter.Project)
		return []backend.StackReference{
			&backend.MockStackReference{NameV: tokens.MustParseStackName("dev")},
		}, nil, nil
	}
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmYes)
	var prompts []string
	var out, errOut bytes.Buffer
	args := newArgs{
		interactive: true,
		// A name given up front can name a project that already has stacks, so the default
		// stack name is checked before the block proposes it.
		name:                 "my-project",
		prompt:               countingPrompt(map[string]string{"Stack name": "prod"}, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
		stderr:               &errOut,
	}

	err := runNew(t.Context(), args)
	require.NoError(t, err)

	assert.Equal(t, []string{"my-project"}, listedProjects)
	assert.Equal(t, []string{"Stack name="}, prompts,
		"a taken stack name is asked for before the block, with no default to accept")
	assert.Contains(t, out.String(), "Stack 'my-org/dev' already exists.")
	assert.Contains(t, out.String(), "Stack name:    my-org/prod")
	require.Equal(t, []string{"my-org/prod"}, *created)
}
