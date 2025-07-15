// Copyright 2016-2024, Pulumi Corporation.
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

package deployment

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type promptAssertion[T any, I any] struct {
	ExpectedDefault T
	Return          I
}

// quick mock, it has a list of values to return for each prompt
type promptHandlersMock struct {
	T                        *testing.T
	ConfirmationResponses    []promptAssertion[bool, bool]
	PromptUserResponses      []promptAssertion[string, string]
	PromptValueResponses     []promptAssertion[string, string]
	PromptUserMultiResponses []promptAssertion[[]string, []string]
	PrintTexts               []string
}

func (p *promptHandlersMock) AssertComplete() {
	assert.Empty(p.T, p.ConfirmationResponses)
	assert.Empty(p.T, p.PromptUserResponses)
	assert.Empty(p.T, p.PromptValueResponses)
	assert.Empty(p.T, p.PromptUserMultiResponses)
	assert.Empty(p.T, p.PrintTexts)
}

func (p *promptHandlersMock) Print(prompt string) {
	if len(p.PrintTexts) == 0 {
		panic(fmt.Sprintf("PrintTexts prompt for %q not found", prompt))
	}
	value := p.PrintTexts[0]
	p.PrintTexts = p.PrintTexts[1:]
	assert.True(p.T, strings.Contains(prompt, value))
}

func (p *promptHandlersMock) AskForConfirmation(
	prompt string, color colors.Colorization, defaultValue bool, yes bool,
) bool {
	if len(p.ConfirmationResponses) == 0 {
		panic(fmt.Sprintf("AskForConfirmation prompt for %q not found", prompt))
	}
	value := p.ConfirmationResponses[0]
	p.ConfirmationResponses = p.ConfirmationResponses[1:]
	assert.Equal(p.T, value.ExpectedDefault, defaultValue)
	return value.Return
}

func (p *promptHandlersMock) PromptUserSkippable(
	yes bool, msg string, options []string, defaultOption string, colorization colors.Colorization,
) string {
	if len(p.PromptUserResponses) == 0 {
		panic(fmt.Sprintf("PromptUserSkippable prompt for %q not found", msg))
	}
	value := p.PromptUserResponses[0]
	p.PromptUserResponses = p.PromptUserResponses[1:]
	assert.Equal(p.T, value.ExpectedDefault, defaultOption)
	return value.Return
}

func (p *promptHandlersMock) PromptUser(
	msg string, options []string, defaultOption string, colorization colors.Colorization,
) string {
	if len(p.PromptUserResponses) == 0 {
		panic(fmt.Sprintf("PromptUser prompt for %q not found", msg))
	}
	value := p.PromptUserResponses[0]
	p.PromptUserResponses = p.PromptUserResponses[1:]
	assert.Equal(p.T, value.ExpectedDefault, defaultOption)
	return value.Return
}

func (p *promptHandlersMock) PromptUserMultiSkippable(
	yes bool, msg string, options []string, defaultOptions []string, colorization colors.Colorization,
) []string {
	if len(p.PromptUserMultiResponses) == 0 {
		panic(fmt.Sprintf("PromptUserMultiSkippable prompt for %q not found", msg))
	}

	value := p.PromptUserMultiResponses[0]
	p.PromptUserMultiResponses = p.PromptUserMultiResponses[1:]
	assert.Equal(p.T, value.ExpectedDefault, defaultOptions)
	return value.Return
}

func (p *promptHandlersMock) PromptForValue(
	yes bool, valueType string, defaultValue string, secret bool,
	isValidFn func(value string) error, opts display.Options,
) (string, error) {
	if len(p.PromptValueResponses) == 0 {
		panic(fmt.Sprintf("PromptForValue prompt for %q not found", valueType))
	}

	value := p.PromptValueResponses[0]
	p.PromptValueResponses = p.PromptValueResponses[1:]
	assert.Equal(p.T, value.ExpectedDefault, defaultValue)
	return value.Return, nil
}

func TestDSConfigureGit(t *testing.T) {
	t.Parallel()

	repoDir := setUpGitWorkspace(context.Background(), t)
	workDir := filepath.Join(repoDir, "goproj")

	t.Run("using the GH app", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := ""

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, true},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", filepath.Join(".", "goproj")},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
			},
			PromptUserMultiResponses: []promptAssertion[[]string, []string]{
				{
					[]string{
						optPreviewPr,
						optUpdatePushes,
					},
					[]string{
						optPreviewPr,
						optUpdatePushes,
						optPrTemplate,
					},
				},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			WorkDir:    workDir,
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
			Backend: &backend.MockBackend{
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: true,
					}, nil
				},
			},
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "pulumi/test-repo", d.Deployment.DeploymentSettings.GitHub.Repository)
		assert.True(t, d.Deployment.DeploymentSettings.GitHub.PreviewPullRequests)
		assert.True(t, d.Deployment.DeploymentSettings.GitHub.DeployCommits)
		assert.True(t, d.Deployment.DeploymentSettings.GitHub.PullRequestTemplate)

		prompts.AssertComplete()
	})

	t.Run("using the GH app already configured", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := ""

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, true},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"test", "goproj"},
				{"staging", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
			},
			PromptUserMultiResponses: []promptAssertion[[]string, []string]{
				{
					[]string{
						optPreviewPr,
						optUpdatePushes,
						optPrTemplate,
					},
					[]string{},
				},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{
					SourceContext: &apitype.SourceContext{
						Git: &apitype.SourceContextGit{
							RepoDir: "test",
							Branch:  "staging",
						},
					},
					GitHub: &apitype.DeploymentSettingsGitHub{
						Repository:          "pulumi/test",
						PreviewPullRequests: true,
						DeployCommits:       true,
						PullRequestTemplate: true,
					},
				},
			},
			WorkDir: workDir,
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
			Backend: &backend.MockBackend{
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: true,
					}, nil
				},
			},
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "pulumi/test-repo", d.Deployment.DeploymentSettings.GitHub.Repository)
		assert.False(t, d.Deployment.DeploymentSettings.GitHub.PreviewPullRequests)
		assert.False(t, d.Deployment.DeploymentSettings.GitHub.DeployCommits)
		assert.False(t, d.Deployment.DeploymentSettings.GitHub.PullRequestTemplate)

		prompts.AssertComplete()
	})

	t.Run("no authentication", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := ""

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, false},
			},
			PromptUserResponses: []promptAssertion[string, string]{
				{optUserPass, optNoAuthentication},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", "goproj"},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{
					SourceContext: &apitype.SourceContext{
						Git: &apitype.SourceContextGit{
							GitAuth: &apitype.GitAuthConfig{
								BasicAuth: &apitype.BasicAuth{
									UserName: apitype.SecretValue{Value: "user"},
									Password: apitype.SecretValue{Ciphertext: "ciphered"},
								},
							},
						},
					},
				},
			},
			WorkDir: workDir,
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
				},
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: true,
					}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "https://github.com/pulumi/test-repo.git", d.Deployment.DeploymentSettings.SourceContext.Git.RepoURL)
		assert.Nil(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth)

		prompts.AssertComplete()
	})

	t.Run("using credentials", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := ""

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, false},
			},
			PromptUserResponses: []promptAssertion[string, string]{
				{optUserPass, optUserPass},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", "goproj"},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
				{"", "username"},
				{"", "password"},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			WorkDir:    workDir,
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
				},
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: true,
					}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "https://github.com/pulumi/test-repo.git", d.Deployment.DeploymentSettings.SourceContext.Git.RepoURL)
		assert.Equal(t, "username", d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
		assert.True(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password.Secret)
		assert.Equal(t, "encrypted", d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password.Ciphertext)

		prompts.AssertComplete()
	})

	t.Run("using ssh", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := "private_key"

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, false},
			},
			PromptUserResponses: []promptAssertion[string, string]{
				{optUserPass, optSSH},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", "goproj"},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
				{"", "password"},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			WorkDir:    workDir,
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					if secret == "private_key" {
						return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
					}
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted_password"}, nil
				},
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: true,
					}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "https://github.com/pulumi/test-repo.git",
			d.Deployment.DeploymentSettings.SourceContext.Git.RepoURL)
		assert.True(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.SSHPrivateKey.Secret)
		assert.Equal(t, "encrypted",
			d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.SSHPrivateKey.Ciphertext)
		assert.True(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password.Secret)
		assert.Equal(t, "encrypted_password",
			d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password.Ciphertext)

		prompts.AssertComplete()
	})

	t.Run("github repo but the app is not installed", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := "private_key"

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, true},
			},
			PromptUserResponses: []promptAssertion[string, string]{
				{optUserPass, optSSH},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", "goproj"},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
				{"", "password"},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app is not installed",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			WorkDir:    workDir,
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					if secret == "private_key" {
						return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
					}
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted_password"}, nil
				},
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: false,
					}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "https://github.com/pulumi/test-repo.git",
			d.Deployment.DeploymentSettings.SourceContext.Git.RepoURL)
		assert.True(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.SSHPrivateKey.Secret)
		assert.Equal(t, "encrypted",
			d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.SSHPrivateKey.Ciphertext)
		assert.True(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password.Secret)
		assert.Equal(t, "encrypted_password",
			d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password.Ciphertext)

		prompts.AssertComplete()
	})

	t.Run("github repo but the app is not installed and aborts", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := "private_key"

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{true, false},
			},
			PromptUserResponses: []promptAssertion[string, string]{},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", "goproj"},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://github.com/pulumi/test-repo.git"},
			},
			PrintTexts: []string{
				"Pulumi’s GitHub app is not installed",
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			WorkDir:    workDir,
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					if secret == "private_key" {
						return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
					}
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted_password"}, nil
				},
				GetGHAppIntegrationF: func(ctx context.Context, stack backend.Stack) (*apitype.GitHubAppIntegration, error) {
					return &apitype.GitHubAppIntegration{
						Installed: false,
					}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		assert.Error(t, err, errAbortCmd.Error())

		prompts.AssertComplete()
	})

	t.Run("non github", func(t *testing.T) {
		t.Parallel()

		gitSSHPrivateKeyPath := ""
		gitSSHPrivateKeyValue := ""

		prompts := &promptHandlersMock{
			T: t,
			PromptUserResponses: []promptAssertion[string, string]{
				{optUserPass, optNoAuthentication},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"goproj", "goproj"},
				{"refs/heads/master", "master"},
				{"https://github.com/pulumi/test-repo.git", "https://example.com/pulumi/test-repo.git"},
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{
					SourceContext: &apitype.SourceContext{
						Git: &apitype.SourceContextGit{
							GitAuth: &apitype.GitAuthConfig{
								BasicAuth: &apitype.BasicAuth{
									UserName: apitype.SecretValue{Value: "user"},
									Password: apitype.SecretValue{Ciphertext: "ciphered"},
								},
							},
						},
					},
				},
			},
			WorkDir: workDir,
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)

		require.NoError(t, err)
		assert.Equal(t, "goproj", d.Deployment.DeploymentSettings.SourceContext.Git.RepoDir)
		assert.Equal(t, "master", d.Deployment.DeploymentSettings.SourceContext.Git.Branch)
		assert.Equal(t, "https://example.com/pulumi/test-repo.git", d.Deployment.DeploymentSettings.SourceContext.Git.RepoURL)
		assert.Nil(t, d.Deployment.DeploymentSettings.SourceContext.Git.GitAuth)

		prompts.AssertComplete()
	})
}

func TestDSConfigureAdvancedSettings(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		prompts := &promptHandlersMock{
			T: t,
			PromptUserMultiResponses: []promptAssertion[[]string, []string]{
				{
					[]string{
						optSkipIntermediateDeployments,
					},
					[]string{
						optSkipIntermediateDeployments,
						optSkipDepsInstall,
					},
				},
			},
		}
		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureAdvancedSettings(d)

		require.NoError(t, err)
		assert.True(t, d.Deployment.DeploymentSettings.Operation.Options.SkipInstallDependencies)
		assert.True(t, d.Deployment.DeploymentSettings.Operation.Options.SkipIntermediateDeployments)

		prompts.AssertComplete()
	})

	t.Run("already configured", func(t *testing.T) {
		t.Parallel()
		prompts := &promptHandlersMock{
			T: t,
			PromptUserMultiResponses: []promptAssertion[[]string, []string]{
				{
					[]string{
						optSkipDepsInstall,
						optSkipIntermediateDeployments,
					},
					[]string{},
				},
			},
		}
		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{
					Operation: &apitype.OperationContext{
						Options: &apitype.OperationContextOptions{
							SkipInstallDependencies:     true,
							SkipIntermediateDeployments: true,
						},
					},
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureAdvancedSettings(d)

		require.NoError(t, err)
		assert.False(t, d.Deployment.DeploymentSettings.Operation.Options.SkipInstallDependencies)
		assert.False(t, d.Deployment.DeploymentSettings.Operation.Options.SkipIntermediateDeployments)

		prompts.AssertComplete()
	})
}

func TestDSConfigureImageRepository(t *testing.T) {
	t.Parallel()

	t.Run("with credentials", func(t *testing.T) {
		t.Parallel()

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{false, true},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"", "image_name"},
				{"", "user"},
				{"", "password"},
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					assert.Equal(t, "password", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted"}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureImageRepository(d)

		require.NoError(t, err)
		assert.Equal(t, "image_name",
			d.Deployment.DeploymentSettings.Executor.ExecutorImage.Reference)
		assert.Equal(t, "user",
			d.Deployment.DeploymentSettings.Executor.ExecutorImage.Credentials.Username)
		assert.Equal(t, "encrypted",
			d.Deployment.DeploymentSettings.Executor.ExecutorImage.Credentials.Password.Ciphertext)

		prompts.AssertComplete()
	})

	t.Run("already configured", func(t *testing.T) {
		t.Parallel()

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{false, true},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"image_name", "image_name_new"},
				{"user", "user_new"},
				{"", "password_new"},
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{
					Executor: &apitype.ExecutorContext{
						ExecutorImage: &apitype.DockerImage{
							Reference: "image_name",
							Credentials: &apitype.DockerImageCredentials{
								Username: "user",
								Password: apitype.SecretValue{
									Ciphertext: "something",
								},
							},
						},
					},
				},
			},
			Backend: &backend.MockBackend{
				EncryptStackDeploymentSettingsSecretF: func(
					ctx context.Context, stack backend.Stack, secret string,
				) (*apitype.SecretValue, error) {
					assert.Equal(t, "password_new", secret)
					return &apitype.SecretValue{Secret: true, Ciphertext: "encrypted_new"}, nil
				},
			},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureImageRepository(d)

		require.NoError(t, err)
		assert.Equal(t, "image_name_new",
			d.Deployment.DeploymentSettings.Executor.ExecutorImage.Reference)
		assert.Equal(t, "user_new",
			d.Deployment.DeploymentSettings.Executor.ExecutorImage.Credentials.Username)
		assert.Equal(t, "encrypted_new",
			d.Deployment.DeploymentSettings.Executor.ExecutorImage.Credentials.Password.Ciphertext)

		prompts.AssertComplete()
	})

	t.Run("without credentials", func(t *testing.T) {
		t.Parallel()

		prompts := &promptHandlersMock{
			T: t,
			ConfirmationResponses: []promptAssertion[bool, bool]{
				{false, true},
			},
			PromptValueResponses: []promptAssertion[string, string]{
				{"", "image_name"},
				{"", ""},
			},
		}

		d := &deploymentSettingsCommandDependencies{
			Deployment: &workspace.ProjectStackDeployment{},
			DisplayOptions: &display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			},
			Prompts: prompts,
		}

		err := configureImageRepository(d)

		require.NoError(t, err)
		assert.Equal(t, "image_name", d.Deployment.DeploymentSettings.Executor.ExecutorImage.Reference)
		assert.Nil(t, d.Deployment.DeploymentSettings.Executor.ExecutorImage.Credentials)

		prompts.AssertComplete()
	})
}
