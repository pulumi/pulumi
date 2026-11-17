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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Ctrl-C is simulated the way the real prompts report it; see isCancelled.

const cancelConfigTemplateYAML = `name: ${PROJECT}
description: ${DESCRIPTION}
runtime: yaml
template:
  description: A cancellation test template
  config:
    aws:region:
      description: The AWS region to deploy into
`

// cancellingPrompt answers text prompts with their defaults and cancels the first prompt whose
// valueType contains cancelAt after skipping skip of them, along with any prompt asked after that.
// When asked is non-nil, it records every prompt.
func cancellingPrompt(cancelAt string, skip int, asked *[]string) promptForValueFunc {
	cancelled := false
	return func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		if asked != nil {
			*asked = append(*asked, valueType)
		}
		if !cancelled && cancelAt != "" && strings.Contains(valueType, cancelAt) {
			if skip == 0 {
				cancelled = true
			}
			skip--
		}
		if cancelled {
			// Cancel again so a command that keeps asking cannot loop.
			return "", io.EOF
		}
		return defaultValue, nil
	}
}

func runtimeOptionsInterrupted(ctx *plugin.Context, language plugin.LanguageRuntime,
	info *workspace.ProjectRuntimeInfo, main string, opts display.Options, yes, interactive bool,
	prompt promptForValueFunc,
) (map[string]any, error) {
	return nil, terminal.InterruptErr
}

// assertCancelled checks, in separate subtests, that a cancelled `pulumi new` left nothing behind and
// said so.
func assertCancelled(t *testing.T, err error, dir string, created []string) {
	t.Helper()

	require.Error(t, err, "a cancelled `pulumi new` must fail") //nolint:forbidigo // pre-existing

	t.Run("nothing left behind", func(t *testing.T) {
		entries, readErr := os.ReadDir(dir)
		require.NoError(t, readErr)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Empty(t, names, "cancelling must not leave project files behind")
		assert.Empty(t, created, "cancelling must not leave a stack behind")
		require.NoError(t, ErrorIfNotEmptyDirectory(dir), "re-running `pulumi new` here must not be blocked")
	})

	t.Run("says no project was created", func(t *testing.T) {
		assert.ErrorIs(t, err, errNoProjectCreated)
	})
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewCancelLeavesNothingBehind(t *testing.T) {
	cases := []struct {
		name string
		// guided runs `pulumi new` with no template argument; otherwise the template is named.
		guided   bool
		template string
		// stack is passed as --stack.
		stack string
		// secretsErr, when set, is returned by the backend's secrets manager, as a cancelled passphrase
		// prompt returns io.EOF.
		secretsErr error
		// cancelAt is a substring of the text prompt to cancel at.
		cancelAt string
		// selects scripts the guided select prompts, including a terminal.InterruptErr to cancel one.
		selects        []any
		runtimeOptions runtimeOptionsFunc
	}{
		{
			name:           "named template/project name",
			template:       localTemplateYAML,
			cancelAt:       "Project name",
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "named template/project description",
			template:       localTemplateYAML,
			cancelAt:       "Project description",
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "guided/confirmation",
			guided:         true,
			template:       guidedConfigTemplateYAML,
			selects:        []any{"AWS", "Python", terminal.InterruptErr},
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "named template/stack name",
			template:       localTemplateYAML,
			cancelAt:       "Stack name",
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "named template/config value",
			template:       cancelConfigTemplateYAML,
			cancelAt:       "(aws:region)",
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "guided with --stack/config value",
			guided:         true,
			template:       cancelConfigTemplateYAML,
			stack:          "dev",
			cancelAt:       "(aws:region)",
			selects:        []any{"AWS", "Python", confirmYes},
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "named template/passphrase",
			template:       localTemplateYAML,
			secretsErr:     io.EOF,
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "named template/runtime option",
			template:       localTemplateYAML,
			runtimeOptions: runtimeOptionsInterrupted,
		},
		{
			name:           "guided/runtime option",
			guided:         true,
			template:       guidedConfigTemplateYAML,
			selects:        []any{"AWS", "Python", confirmYes},
			runtimeOptions: runtimeOptionsInterrupted,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempProjectDir(t)
			t.Chdir(dir)

			var templateArg string
			if tc.guided {
				guidedRepoTemplate(t, "aws-python", tc.template)
			} else {
				templateArg = writeLocalTemplate(t, t.TempDir(), "template", tc.template)
			}

			b, created := guidedNewTestBackend(t)
			if tc.secretsErr != nil {
				b.DefaultSecretManagerF = func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
					return nil, tc.secretsErr
				}
			}
			mockCurrentBackend(t, b)

			selectOne, _ := scriptedSelect(t, tc.selects...)
			var out, errOut bytes.Buffer
			err := runNew(t.Context(), newArgs{
				interactive:          true,
				templateNameOrURL:    templateArg,
				stack:                tc.stack,
				prompt:               cancellingPrompt(tc.cancelAt, 0, nil),
				promptRuntimeOptions: tc.runtimeOptions,
				languageTemplate:     languageTemplateMock,
				selectOne:            selectOne,
				secretsProvider:      "default",
				stdout:               &out,
				stderr:               &errOut,
			})

			assertCancelled(t, err, dir, *created)
		})
	}
}

// Cancelling a DIY backend's passphrase prompt surfaces as io.EOF from creating the secrets manager. It
// must end `pulumi new` instead of asking for another stack name. With --force, the passphrase is asked
// as the stack is created, where a failure could otherwise ask for another stack name.
//
//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewCancelAtPassphrase(t *testing.T) {
	dir := tempProjectDir(t)
	t.Chdir(dir)
	templateArg := writeLocalTemplate(t, t.TempDir(), "template", localTemplateYAML)

	b, created := guidedNewTestBackend(t)
	b.DefaultSecretManagerF = func(ctx context.Context, ps *workspace.ProjectStack) (secrets.Manager, error) {
		return nil, io.EOF
	}
	mockCurrentBackend(t, b)

	// Answer the first stack-name prompt, so only the passphrase is cancelled, and cancel any
	// second one so a retry loop ends.
	var asked []string
	var out, errOut bytes.Buffer
	err := runNew(t.Context(), newArgs{
		interactive:          true,
		force:                true,
		templateNameOrURL:    templateArg,
		prompt:               cancellingPrompt("Stack name", 1, &asked),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		secretsProvider:      "default",
		stdout:               &out,
		stderr:               &errOut,
	})

	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, []string{"Project name", "Project description", "Stack name"}, asked,
		"cancelling the passphrase must not ask for another stack name; output:\n%s", out.String())
	assert.Empty(t, *created, "cancelling the passphrase must not create a stack")
}

// A DIY backend resolves a stack name without a project against its current project, which
// `pulumi new` hasn't saved yet when the guided flow asks for a stack name.
//
//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewValidatesStackNameAgainstNewProject(t *testing.T) {
	cases := []struct {
		name string
		// projectName is passed as --name, so the project counts as existing and its stacks are checked.
		projectName string
		// takenStacks are the project's existing stacks.
		takenStacks []string
		confirm     string
	}{
		{name: "after Change these values", confirm: confirmChange},
		{name: "default stack taken", projectName: "proj", takenStacks: []string{"dev"}, confirm: confirmYes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guidedRepoTemplate(t, "aws-python", guidedConfigTemplateYAML)
			dir := tempProjectDir(t)
			t.Chdir(dir)

			b, created := guidedNewTestBackend(t)
			var current *workspace.Project
			b.SetCurrentProjectF = func(proj *workspace.Project) { current = proj }
			// As the DIY backend does (see diyBackend.parseStackReference).
			b.ValidateStackNameF = func(name string) error {
				if !strings.Contains(name, "/") && current == nil {
					return errors.New("no Pulumi.yaml project file found")
				}
				return nil
			}
			b.ListStackNamesF = func(ctx context.Context, filter backend.ListStackNamesFilter,
				token backend.ContinuationToken,
			) ([]backend.StackReference, backend.ContinuationToken, error) {
				refs := make([]backend.StackReference, 0, len(tc.takenStacks))
				for _, name := range tc.takenStacks {
					refs = append(refs, &backend.MockStackReference{NameV: tokens.MustParseStackName(name)})
				}
				return refs, nil, nil
			}
			mockCurrentBackend(t, b)

			selectOne, _ := scriptedSelect(t, "AWS", "Python", tc.confirm)
			var prompts []string
			err := runNew(t.Context(), newArgs{
				interactive:          true,
				name:                 tc.projectName,
				prompt:               countingPrompt(map[string]string{"Stack name": "prod"}, &prompts),
				promptRuntimeOptions: runtimeOptionsNone,
				languageTemplate:     languageTemplateMock,
				selectOne:            selectOne,
				secretsProvider:      "default",
			})

			require.NoError(t, err)
			assert.Equal(t, []string{"my-org/prod"}, *created)
		})
	}
}

// Config asked before the files are copied must still reach the new stack's config file.
//
//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewNamedTemplateSavesConfigAskedUpFront(t *testing.T) {
	dir := tempProjectDir(t)
	t.Chdir(dir)
	templateArg := writeLocalTemplate(t, t.TempDir(), "template", cancelConfigTemplateYAML)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	var prompts []string
	answers := map[string]string{"The AWS region to deploy into (aws:region)": "us-west-2"}
	err := runNew(t.Context(), newArgs{
		interactive:          true,
		templateNameOrURL:    templateArg,
		configArray:          []string{"aws:profile=work"},
		prompt:               countingPrompt(answers, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		secretsProvider:      "default",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"my-org/dev"}, *created)

	proj := loadProject(t, dir)
	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	for key, want := range map[string]string{"region": "us-west-2", "profile": "work"} {
		v, ok := projStack.Config[config.MustMakeKey("aws", key)]
		require.True(t, ok, "aws:%s must be saved", key)
		got, err := v.Value(config.NopDecrypter)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

// The guided confirmation doesn't show config when --stack names the stack, so config is asked after it
// and must still reach the new stack's config file.
//
//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewWithStackSavesConfigAskedUpFront(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", cancelConfigTemplateYAML)
	dir := tempProjectDir(t)
	t.Chdir(dir)

	b, created := guidedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmYes)
	var prompts []string
	answers := map[string]string{"The AWS region to deploy into (aws:region)": "us-west-2"}
	err := runNew(t.Context(), newArgs{
		interactive:          true,
		stack:                "dev",
		prompt:               countingPrompt(answers, &prompts),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"my-org/dev"}, *created)

	proj := loadProject(t, dir)
	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	v, ok := projStack.Config[config.MustMakeKey("aws", "region")]
	require.True(t, ok, "aws:region must be saved")
	got, err := v.Value(config.NopDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", got)
}

// The secrets manager prepared with the other questions must still be recorded in the new stack's
// config file.
//
//nolint:paralleltest // changes directory for process, sets environment variables
func TestNewRecordsPassphraseAskedUpFront(t *testing.T) {
	useTempFilestateBackend(t)
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := NewNewCmd()
	cmd.SetArgs([]string{"-y", "--name", "my-project", "--stack", "dev"})

	require.NoError(t, cmd.Execute())

	proj := loadProject(t, dir)
	projStack, err := workspace.LoadProjectStack(nil /*sink*/, proj, filepath.Join(dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	assert.NotEmpty(t, projStack.EncryptionSalt, "the passphrase secrets manager must be saved with the stack")
}

// With --force, the stack's config file may already have secrets settings, so a secrets manager prepared
// with the other questions would ask for a passphrase only to discard it.
//
//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewForceDoesNotPrepareSecretsEarly(t *testing.T) {
	dir := tempProjectDir(t)
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.dev.yaml"), []byte("encryptionsalt: v1:abc\n"), 0o600))
	templateArg := writeLocalTemplate(t, t.TempDir(), "template", localTemplateYAML)

	b, created := guidedNewTestBackend(t)
	secretsManagers := 0
	b.DefaultSecretManagerF = func(ctx context.Context, ps *workspace.ProjectStack) (secrets.Manager, error) {
		secretsManagers++
		return nil, nil
	}
	mockCurrentBackend(t, b)

	err := runNew(t.Context(), newArgs{
		interactive:          true,
		force:                true,
		templateNameOrURL:    templateArg,
		prompt:               cancellingPrompt("" /*cancelAt*/, 0, nil),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		secretsProvider:      "default",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"my-org/dev"}, *created)
	assert.Equal(t, 1, secretsManagers, "only stack creation, which reads Pulumi.dev.yaml, must create a secrets manager")
}
