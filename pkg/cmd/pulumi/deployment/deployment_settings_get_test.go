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

package deployment

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDeploymentSettingsGetClient stubs deploymentSettingsGetClient with a
// fixed response (or error).
type mockDeploymentSettingsGetClient struct {
	resp *apitype.DeploymentSettings
	err  error
}

func (m *mockDeploymentSettingsGetClient) GetStackDeploymentSettings(
	_ context.Context, _ client.StackIdentifier,
) (*apitype.DeploymentSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func stubSettingsGetFactory(c deploymentSettingsGetClient) deploymentSettingsGetClientFactory {
	return func(_ context.Context, _ string) (deploymentSettingsGetClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingSettingsGetFactory(err error) deploymentSettingsGetClientFactory {
	return func(_ context.Context, _ string) (deploymentSettingsGetClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

func deploymentSettingsGetJSONArgs(t *testing.T) deploymentSettingsGetArgs {
	t.Helper()
	args := deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	return args
}

func sampleDeploymentSettings() *apitype.DeploymentSettings {
	agentPool := "pool-1"
	return &apitype.DeploymentSettings{
		Executor: &apitype.ExecutorContext{
			WorkingDirectory: "/work",
			ExecutorImage:    &apitype.DockerImage{Reference: "pulumi/pulumi:latest"},
		},
		SourceContext: &apitype.SourceContext{
			Git: &apitype.SourceContextGit{
				RepoURL: "https://github.com/acme/infra",
				Branch:  "main",
				Commit:  "abc123",
				RepoDir: "stacks/prod",
			},
		},
		GitHub: &apitype.DeploymentSettingsGitHub{
			Repository:          "acme/infra",
			DeployCommits:       true,
			PreviewPullRequests: true,
			PullRequestTemplate: false,
			Paths:               []string{"stacks/prod/**"},
		},
		Operation: &apitype.OperationContext{
			Operation:      apitype.Update,
			PreRunCommands: []string{"echo hi"},
			EnvironmentVariables: map[string]apitype.SecretValue{
				"FOO":     {Value: "bar"},
				"BAZ":     {Value: "qux"},
				"API_KEY": {Value: "s3cret", Secret: true},
			},
		},
		AgentPoolID: &agentPool,
	}
}

func TestDeploymentSettingsGet_DefaultOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: sampleDeploymentSettings()}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `Source: GitHub
  Repository:           acme/infra
  Branch:               main
  Commit:               abc123
  Pulumi.yaml folder:   stacks/prod
  Run previews for PRs: yes
  Run updates on push:  yes
  PR stack template:    no
  Path filters:         stacks/prod/**

Deployment runner
  Runner pool:          pool-1
  Executor image:       pulumi/pulumi:latest

Pre-run commands
  echo hi

Environment variables
  API_KEY:              [secret]
  BAZ:                  qux
  FOO:                  bar
`, buf.String())
}

func TestDeploymentSettingsGet_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: &apitype.DeploymentSettings{}}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.NoError(t, err)

	// Nothing configured, hide all sections
	assert.Equal(t, "No deployment settings are configured for this stack.\n", buf.String())
}

// The factory has already resolved the stack, so a 404 means the stack has no settings rather than
// that the stack is missing.
func TestDeploymentSettingsGet_NotFoundRendersEmpty(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsGetClient{
		err: &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"},
	}

	var buf bytes.Buffer
	require.NoError(t, runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()}))
	assert.Equal(t, "No deployment settings are configured for this stack.\n", buf.String())

	var jsonOut bytes.Buffer
	require.NoError(t, runDeploymentSettingsGet(t.Context(), &jsonOut, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t)))
	assert.JSONEq(t, `{}`, jsonOut.String())
}

// Every other status stays fatal.
func TestDeploymentSettingsGet_OtherErrorsAreFatal(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsGetClient{
		err: &apitype.ErrorResponse{Code: http.StatusInternalServerError, Message: "boom"},
	}

	var buf bytes.Buffer
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment settings")
}

func TestDeploymentSettingsGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: sampleDeploymentSettings()}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"source": {
			"kind": "github",
			"repository": "acme/infra",
			"branch": "main",
			"commit": "abc123",
			"folder": "stacks/prod",
			"previewPullRequests": true,
			"runUpdatesOnPush": true,
			"pullRequestTemplate": false,
			"pathFilters": ["stacks/prod/**"]
		},
		"runner": {
			"pool": "pool-1",
			"executorImage": "pulumi/pulumi:latest"
		},
		"preRunCommands": ["echo hi"],
		"environmentVariables": [
			{"name": "API_KEY", "secret": true},
			{"name": "BAZ", "value": "qux"},
			{"name": "FOO", "value": "bar"}
		]
	}`, buf.String())
}

func TestDeploymentSettingsGet_JSONOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: &apitype.DeploymentSettings{}}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{}`, buf.String())
}

func TestDeploymentSettingsGet_JSONOutput_GitSource(t *testing.T) {
	t.Parallel()

	// No GitHub block. The raw git source falls through to source.kind == "git",
	// and the GitHub-only toggles must NOT appear in the JSON.
	settings := &apitype.DeploymentSettings{
		SourceContext: &apitype.SourceContext{
			Git: &apitype.SourceContextGit{
				RepoURL: "git@example.com:acme/infra.git",
				Branch:  "main",
			},
		},
	}

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: settings}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"source": {
			"kind": "git",
			"repository": "git@example.com:acme/infra.git",
			"branch": "main"
		}
	}`, buf.String())
}

func TestDeploymentSettingsGet_RichSections(t *testing.T) {
	t.Parallel()

	thirtyMin, err := time.ParseDuration("30m")
	require.NoError(t, err)
	fifteenMin, err := time.ParseDuration("15m")
	require.NoError(t, err)
	settings := &apitype.DeploymentSettings{
		Tag: "rev-42",
		Operation: &apitype.OperationContext{
			Operation: apitype.Update,
			OIDC: &apitype.OperationContextOIDCConfiguration{
				AWS: &apitype.OperationContextAWSOIDCConfiguration{
					RoleARN:     "arn:aws:iam::123:role/pulumi-deploy",
					SessionName: "pulumi-deploy",
					Duration:    apitype.DeploymentDuration(thirtyMin),
					PolicyARNs:  []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
				},
				Azure: &apitype.OperationContextAzureOIDCConfiguration{
					ClientID:       "client-1",
					TenantID:       "tenant-1",
					SubscriptionID: "sub-1",
				},
				GCP: &apitype.OperationContextGCPOIDCConfiguration{
					ProjectID:      "123456",
					WorkloadPoolID: "pulumi-pool",
					ProviderID:     "pulumi",
					ServiceAccount: "pulumi@my-project.iam.gserviceaccount.com",
					Region:         "us-central1",
					TokenLifetime:  apitype.DeploymentDuration(fifteenMin),
				},
			},
			Options: &apitype.OperationContextOptions{
				SkipInstallDependencies:     true,
				SkipIntermediateDeployments: true,
				Shell:                       "bash",
				DeleteAfterDestroy:          true,
				RemediateIfDriftDetected:    true,
			},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Tag:                             rev-42

OIDC
  AWS
    Role ARN:                    arn:aws:iam::123:role/pulumi-deploy
    Session name:                pulumi-deploy
    Session duration:            30m0s
    Policy ARNs:                 arn:aws:iam::aws:policy/ReadOnlyAccess
  Azure
    Client ID:                   client-1
    Tenant ID:                   tenant-1
    Subscription ID:             sub-1
  GCP
    Project number:              123456
    Workload pool:               pulumi-pool
    Provider:                    pulumi
    Service account:             pulumi@my-project.iam.gserviceaccount.com
    Region:                      us-central1
    Token lifetime:              15m0s

Advanced
  Skip install dependencies:     yes
  Skip intermediate deployments: yes
  Shell:                         bash
  Delete after destroy:          yes
  Remediate on drift:            yes
`, text)

	assert.JSONEq(t, `{
		"tag": "rev-42",
		"oidc": {
			"aws": {
				"roleArn": "arn:aws:iam::123:role/pulumi-deploy",
				"sessionName": "pulumi-deploy",
				"sessionDuration": "30m0s",
				"policyArns": ["arn:aws:iam::aws:policy/ReadOnlyAccess"]
			},
			"azure": {
				"clientId": "client-1",
				"tenantId": "tenant-1",
				"subscriptionId": "sub-1"
			},
			"gcp": {
				"projectNumber": "123456",
				"workloadPoolId": "pulumi-pool",
				"providerId": "pulumi",
				"serviceAccount": "pulumi@my-project.iam.gserviceaccount.com",
				"region": "us-central1",
				"tokenLifetime": "15m0s"
			}
		},
		"advanced": {
			"skipInstallDependencies": true,
			"skipIntermediateDeployments": true,
			"shell": "bash",
			"deleteAfterDestroy": true,
			"remediateIfDriftDetected": true
		}
	}`, jsonOut)
}

// renderBoth runs the same settings through the text and JSON renderers.
func renderBoth(t *testing.T, settings *apitype.DeploymentSettings) (string, string) {
	t.Helper()
	c := &mockDeploymentSettingsGetClient{resp: settings}

	var text bytes.Buffer
	require.NoError(t, runDeploymentSettingsGet(t.Context(), &text, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()}))

	var jsonOut bytes.Buffer
	require.NoError(t, runDeploymentSettingsGet(t.Context(), &jsonOut, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t)))

	return text.String(), jsonOut.String()
}

func TestDeploymentSettingsGet_VCSProviders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		provider apitype.VCSProvider
		heading  string
	}{
		{apitype.VCSProviderGitHub, "Source: GitHub"},
		{apitype.VCSProviderGitLab, "Source: GitLab"},
		{apitype.VCSProviderAzureDevOps, "Source: Azure DevOps"},
		{apitype.VCSProviderBitbucket, "Source: Bitbucket"},
		{apitype.VCSProviderCustom, "Source: Custom"},
	}

	for _, tc := range cases {
		t.Run(string(tc.provider), func(t *testing.T) {
			t.Parallel()

			settings := &apitype.DeploymentSettings{
				VCS: &apitype.DeploymentSettingsVCS{
					Provider:            tc.provider,
					Repository:          "acme/infra",
					InstallationID:      "inst-7",
					DeployCommits:       true,
					PreviewPullRequests: true,
					DeployTags:          true,
					TagFilters:          []string{"v*"},
					Paths:               []string{"stacks/prod/**"},
				},
				SourceContext: &apitype.SourceContext{
					Git: &apitype.SourceContextGit{Branch: "main"},
				},
			}

			text, jsonOut := renderBoth(t, settings)

			assert.Equal(t, tc.heading+`
  Repository:           acme/infra
  Installation ID:      inst-7
  Branch:               main
  Run previews for PRs: yes
  Run updates on push:  yes
  PR stack template:    no
  Deploy on tag:        yes
  Tag filters:          v*
  Path filters:         stacks/prod/**
`, text)

			assert.JSONEq(t, `{
				"source": {
					"kind": "`+string(tc.provider)+`",
					"repository": "acme/infra",
					"installationId": "inst-7",
					"branch": "main",
					"previewPullRequests": true,
					"runUpdatesOnPush": true,
					"pullRequestTemplate": false,
					"deployTags": true,
					"tagFilters": ["v*"],
					"pathFilters": ["stacks/prod/**"]
				}
			}`, jsonOut)
		})
	}
}

func TestDeploymentSettingsGet_ReviewStackLabelsAreGitHubOnly(t *testing.T) {
	t.Parallel()

	vcs := func(p apitype.VCSProvider) *apitype.DeploymentSettings {
		return &apitype.DeploymentSettings{
			VCS: &apitype.DeploymentSettingsVCS{
				Provider:          p,
				Repository:        "acme/infra",
				ReviewStackLabels: []string{"deploy", "preview"},
			},
		}
	}

	text, jsonOut := renderBoth(t, vcs(apitype.VCSProviderGitHub))
	assert.Regexp(t, `\n  Review stack labels: +deploy, preview\n`, text)
	assert.Contains(t, jsonOut, `"reviewStackLabels"`)

	text, jsonOut = renderBoth(t, vcs(apitype.VCSProviderGitLab))
	assert.NotContains(t, text, "Review stack labels")
	assert.NotContains(t, jsonOut, "reviewStackLabels")
}

func TestDeploymentSettingsGet_LegacyGitHubFields(t *testing.T) {
	t.Parallel()

	deployPR := int64(42)
	settings := &apitype.DeploymentSettings{
		GitHub: &apitype.DeploymentSettingsGitHub{
			Repository:        "acme/infra",
			InstallationID:    "inst-7",
			DeployPullRequest: &deployPR,
			DeployTags:        true,
			TagFilters:        []string{"v*", "release-*"},
			ReviewStackLabels: []string{"deploy"},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Source: GitHub
  Repository:           acme/infra
  Installation ID:      inst-7
  Run previews for PRs: no
  Run updates on push:  no
  PR stack template:    no
  Deploy PR:            42
  Deploy on tag:        yes
  Tag filters:          v*, release-*
  Review stack labels:  deploy
`, text)

	assert.JSONEq(t, `{
		"source": {
			"kind": "github",
			"repository": "acme/infra",
			"installationId": "inst-7",
			"previewPullRequests": false,
			"runUpdatesOnPush": false,
			"pullRequestTemplate": false,
			"deployPullRequest": 42,
			"deployTags": true,
			"tagFilters": ["v*", "release-*"],
			"reviewStackLabels": ["deploy"]
		}
	}`, jsonOut)
}

func TestDeploymentSettingsGet_VCSTakesPrecedenceOverLegacyGitHub(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		GitHub: &apitype.DeploymentSettingsGitHub{Repository: "acme/legacy"},
		VCS: &apitype.DeploymentSettingsVCS{
			Provider:   apitype.VCSProviderGitLab,
			Repository: "acme/current",
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Source: GitLab
  Repository:           acme/current
  Run previews for PRs: no
  Run updates on push:  no
  PR stack template:    no
`, text)
	assert.NotContains(t, jsonOut, "acme/legacy")
}

func TestDeploymentSettingsGet_BranchStripsRefsHeads(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		SourceContext: &apitype.SourceContext{
			Git: &apitype.SourceContextGit{
				RepoURL: "https://example.com/acme/infra",
				Branch:  "refs/heads/release/1.0",
			},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Source: Git
  Repository: https://example.com/acme/infra
  Branch:     release/1.0
`, text)

	assert.JSONEq(t, `{
		"source": {
			"kind": "git",
			"repository": "https://example.com/acme/infra",
			"branch": "release/1.0"
		}
	}`, jsonOut)
}

func TestDeploymentSettingsGet_GitTag(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		SourceContext: &apitype.SourceContext{
			Git: &apitype.SourceContextGit{
				RepoURL: "https://example.com/acme/infra",
				Commit:  "abc123",
				Tag:     "v1.2.3",
			},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Source: Git
  Repository: https://example.com/acme/infra
  Commit:     abc123
  Git tag:    v1.2.3
`, text)
	assert.JSONEq(t, `{
		"source": {
			"kind": "git",
			"repository": "https://example.com/acme/infra",
			"commit": "abc123",
			"gitTag": "v1.2.3"
		}
	}`, jsonOut)
}

func TestDeploymentSettingsGet_GitAuthModes(t *testing.T) {
	t.Parallel()

	sshOnly := &apitype.GitAuthConfig{
		SSHAuth: &apitype.SSHAuth{SSHPrivateKey: apitype.SecretValue{Value: "PRIVATE-KEY", Secret: true}},
	}
	tokenOnly := &apitype.GitAuthConfig{
		PersonalAccessToken: &apitype.SecretValue{Value: "TOKEN", Secret: true},
	}
	basicOnly := &apitype.GitAuthConfig{
		BasicAuth: &apitype.BasicAuth{
			UserName: apitype.SecretValue{Value: "user"},
			Password: apitype.SecretValue{Value: "PASSWORD", Secret: true},
		},
	}

	cases := []struct {
		name string
		auth *apitype.GitAuthConfig
		want string
	}{
		{"ssh", sshOnly, "SSH key"},
		{"token", tokenOnly, "Access token"},
		{"basic", basicOnly, "Basic auth"},
		{"ssh wins over token and basic", &apitype.GitAuthConfig{
			SSHAuth:             sshOnly.SSHAuth,
			PersonalAccessToken: tokenOnly.PersonalAccessToken,
			BasicAuth:           basicOnly.BasicAuth,
		}, "SSH key"},
		{"token wins over basic", &apitype.GitAuthConfig{
			PersonalAccessToken: tokenOnly.PersonalAccessToken,
			BasicAuth:           basicOnly.BasicAuth,
		}, "Access token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			settings := &apitype.DeploymentSettings{
				SourceContext: &apitype.SourceContext{
					Git: &apitype.SourceContextGit{
						RepoURL: "https://example.com/acme/infra",
						GitAuth: tc.auth,
					},
				},
			}

			text, jsonOut := renderBoth(t, settings)

			assert.Equal(t, `Source: Git
  Repository:     https://example.com/acme/infra
  Authentication: `+tc.want+"\n", text)
			assert.JSONEq(t, `{
				"source": {
					"kind": "git",
					"repository": "https://example.com/acme/infra",
					"auth": "`+tc.want+`"
				}
			}`, jsonOut)

			for _, material := range []string{"PRIVATE-KEY", "TOKEN", "PASSWORD"} {
				assert.NotContains(t, text, material)
				assert.NotContains(t, jsonOut, material)
			}
		})
	}
}

func TestDeploymentSettingsGet_MercurialSource(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		SourceContext: &apitype.SourceContext{
			Hg: &apitype.SourceContextHg{
				RepoURL:  "https://hg.example.com/acme/infra",
				Branch:   "refs/heads/default",
				Revision: "9f2c4a1",
				RepoDir:  "stacks/prod",
				HgAuth: &apitype.GitAuthConfig{
					BasicAuth: &apitype.BasicAuth{
						UserName: apitype.SecretValue{Value: "user"},
						Password: apitype.SecretValue{Value: "PASSWORD", Secret: true},
					},
				},
			},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Source: Mercurial
  Repository:         https://hg.example.com/acme/infra
  Branch:             default
  Revision:           9f2c4a1
  Pulumi.yaml folder: stacks/prod
  Authentication:     Basic auth
`, text)

	assert.JSONEq(t, `{
		"source": {
			"kind": "hg",
			"repository": "https://hg.example.com/acme/infra",
			"branch": "default",
			"revision": "9f2c4a1",
			"folder": "stacks/prod",
			"auth": "Basic auth"
		}
	}`, jsonOut)
	assert.NotContains(t, jsonOut, "PASSWORD")
}

// The service accepts a vcs integration on top of a Mercurial checkout, so the checkout details
// still have to render.
func TestDeploymentSettingsGet_VCSWithMercurialCheckout(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		VCS: &apitype.DeploymentSettingsVCS{
			Provider:   apitype.VCSProviderCustom,
			Repository: "acme/infra",
		},
		SourceContext: &apitype.SourceContext{
			Hg: &apitype.SourceContextHg{
				RepoURL:  "https://hg.example.com/acme/infra",
				Branch:   "refs/heads/default",
				Revision: "9f2c4a1",
				RepoDir:  "stacks/prod",
			},
		},
	}

	_, jsonOut := renderBoth(t, settings)

	assert.JSONEq(t, `{
		"source": {
			"kind": "custom",
			"repository": "acme/infra",
			"branch": "default",
			"revision": "9f2c4a1",
			"folder": "stacks/prod",
			"previewPullRequests": false,
			"runUpdatesOnPush": false,
			"pullRequestTemplate": false
		}
	}`, jsonOut)
}

func TestDeploymentSettingsGet_TemplateSource(t *testing.T) {
	t.Parallel()

	t.Run("explicit source url", func(t *testing.T) {
		t.Parallel()

		settings := &apitype.DeploymentSettings{
			SourceContext: &apitype.SourceContext{
				Template: &apitype.SourceContextTemplate{
					SourceURL:        "registry://templates/source/acme/base@1.0.0",
					ProjectSourceURL: "https://example.com/acme/templates",
				},
			},
		}

		text, jsonOut := renderBoth(t, settings)

		assert.Equal(t, `Source: Template
  Template source: registry://templates/source/acme/base@1.0.0
`, text)
		assert.JSONEq(t, `{
			"source": {
				"kind": "template",
				"templateSourceUrl": "registry://templates/source/acme/base@1.0.0"
			}
		}`, jsonOut)
	})

	t.Run("inherited from project", func(t *testing.T) {
		t.Parallel()

		settings := &apitype.DeploymentSettings{
			SourceContext: &apitype.SourceContext{
				Template: &apitype.SourceContextTemplate{
					ProjectSourceURL: "https://example.com/acme/templates",
					GitAuth: &apitype.GitAuthConfig{
						PersonalAccessToken: &apitype.SecretValue{Value: "TOKEN", Secret: true},
					},
				},
			},
		}

		text, jsonOut := renderBoth(t, settings)

		assert.Equal(t, `Source: Template
  Project template source: https://example.com/acme/templates
  Authentication:          Access token
`, text)
		assert.JSONEq(t, `{
			"source": {
				"kind": "template",
				"projectTemplateSourceUrl": "https://example.com/acme/templates",
				"auth": "Access token"
			}
		}`, jsonOut)
	})
}

func TestDeploymentSettingsGet_EnvironmentVariableValues(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		Operation: &apitype.OperationContext{
			EnvironmentVariables: map[string]apitype.SecretValue{
				"LOG_LEVEL": {Value: "info"},
				"API_KEY":   {Value: "API-KEY-PLAINTEXT", Secret: true},
				"DB_PASS":   {Ciphertext: "AQID", Secret: true},
			},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Environment variables
  API_KEY:   [secret]
  DB_PASS:   [secret]
  LOG_LEVEL: info
`, text)

	assert.JSONEq(t, `{
		"environmentVariables": [
			{"name": "API_KEY", "secret": true},
			{"name": "DB_PASS", "secret": true},
			{"name": "LOG_LEVEL", "value": "info"}
		]
	}`, jsonOut)
	assert.NotContains(t, jsonOut, "AQID")
	assert.NotContains(t, jsonOut, "API-KEY-PLAINTEXT")
	assert.NotContains(t, text, "API-KEY-PLAINTEXT")
}

func TestDeploymentSettingsGet_ValueColumnFitsPrintedLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		vars map[string]apitype.SecretValue
		want string
	}{
		{
			name: "the column shrinks to the labels present",
			vars: map[string]apitype.SecretValue{"FOO": {Value: "bar"}},
			want: `Source: Git
  Repository: https://example.com/acme/infra

Environment variables
  FOO:        bar
`,
		},
		{
			name: "labels are measured by display width",
			vars: map[string]apitype.SecretValue{"ラベル": {Value: "bar"}},
			want: `Source: Git
  Repository: https://example.com/acme/infra

Environment variables
  ラベル:     bar
`,
		},
		{
			name: "a label past the old width overflows its own line",
			vars: map[string]apitype.SecretValue{"A_VERY_LONG_ENVIRONMENT_VARIABLE_NAME": {Value: "bar"}},
			want: `Source: Git
  Repository:                    https://example.com/acme/infra

Environment variables
  A_VERY_LONG_ENVIRONMENT_VARIABLE_NAME: bar
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			text, _ := renderBoth(t, &apitype.DeploymentSettings{
				SourceContext: &apitype.SourceContext{
					Git: &apitype.SourceContextGit{RepoURL: "https://example.com/acme/infra"},
				},
				Operation: &apitype.OperationContext{EnvironmentVariables: tc.vars},
			})

			assert.Equal(t, tc.want, text)
		})
	}
}

func TestDeploymentSettingsGet_ExecutorImageDetails(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		Executor: &apitype.ExecutorContext{
			ExecutorImage: &apitype.DockerImage{
				Reference: "pulumi/pulumi:latest",
				IsDefault: true,
				Credentials: &apitype.DockerImageCredentials{
					Username: "registry-user",
					Password: apitype.SecretValue{Value: "REGISTRY-PASSWORD", Secret: true},
				},
			},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Deployment runner
  Executor image:    pulumi/pulumi:latest
  Default image:     yes
  Image credentials: configured
`, text)

	assert.JSONEq(t, `{
		"runner": {
			"executorImage": "pulumi/pulumi:latest",
			"defaultImage": true,
			"imageCredentials": true
		}
	}`, jsonOut)
	assert.NotContains(t, jsonOut, "REGISTRY-PASSWORD")
	assert.NotContains(t, jsonOut, "registry-user")
}

func TestDeploymentSettingsGet_SettingsMetadata(t *testing.T) {
	t.Parallel()

	source := apitype.DeploymentSettingsSourceGitHubReviewStack
	settings := &apitype.DeploymentSettings{
		Tag:            "rev-42",
		Version:        7,
		SettingsSource: &source,
		CacheOptions:   &apitype.CacheOptions{Enable: true},
		Operation: &apitype.OperationContext{
			Role: &apitype.DeploymentRole{ID: "role-1", Name: "prod-deployer"},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Tag:              rev-42
Version:          7
Settings source:  github-review-stack
Deployment role:  prod-deployer (role-1)
Dependency cache: enabled
`, text)

	assert.JSONEq(t, `{
		"tag": "rev-42",
		"version": 7,
		"settingsSource": "github-review-stack",
		"deploymentRole": {"id": "role-1", "name": "prod-deployer"},
		"cacheEnabled": true
	}`, jsonOut)
}

// The service annotates an assigned role with its name on every read, so a role that arrives
// without one is a resolution failure rather than an unassigned role: render the id, never wording
// that would read as "no role".
func TestDeploymentSettingsGet_DeploymentRoleWithoutName(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		Operation: &apitype.OperationContext{
			Role: &apitype.DeploymentRole{ID: "role-1"},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, "Deployment role: role-1\n", text)
	assert.JSONEq(t, `{"deploymentRole": {"id": "role-1"}}`, jsonOut)
}

func TestDeploymentSettingsGet_CacheDisabled(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{CacheOptions: &apitype.CacheOptions{Enable: false}}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, "Dependency cache: disabled\n", text)
	assert.JSONEq(t, `{"cacheEnabled": false}`, jsonOut)
}

func TestDeploymentSettingsGet_RemediateIfDriftDetectedOnly(t *testing.T) {
	t.Parallel()

	settings := &apitype.DeploymentSettings{
		Operation: &apitype.OperationContext{
			Options: &apitype.OperationContextOptions{RemediateIfDriftDetected: true},
		},
	}

	text, jsonOut := renderBoth(t, settings)

	assert.Equal(t, `Advanced
  Remediate on drift: yes
`, text)
	assert.JSONEq(t, `{"advanced": {"remediateIfDriftDetected": true}}`, jsonOut)
}

func TestDeploymentSettingsGet_ClientError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{err: errors.New("boom")}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment settings")
	assert.Contains(t, err.Error(), "boom")
}

func TestDeploymentSettingsGet_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentSettingsGet(t.Context(), &buf,
		failingSettingsGetFactory(errors.New("not logged in")),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}
