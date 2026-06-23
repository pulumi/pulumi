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
				"FOO": {Value: "bar"},
				"BAZ": {Value: "qux"},
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
  Repository:               acme/infra
  Branch:                   main
  Commit:                   abc123
  Pulumi.yaml folder:       stacks/prod
  Run previews for PRs:     yes
  Run updates on push:      yes
  PR stack template:        no
  Path filters:             stacks/prod/**

Deployment runner
  Runner pool:              pool-1
  Executor image:           pulumi/pulumi:latest

Pre-run commands
  echo hi

Environment variables
  BAZ
  FOO
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
	assert.Empty(t, buf.String())
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
		"environmentVariables": ["BAZ", "FOO"]
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
				GCP: &apitype.OperationContextGCPOIDCConfiguration{
					ProjectID:      "123456",
					WorkloadPoolID: "pulumi-pool",
					ProviderID:     "pulumi",
					ServiceAccount: "pulumi@my-project.iam.gserviceaccount.com",
				},
			},
			Options: &apitype.OperationContextOptions{
				SkipInstallDependencies: true,
				Shell:                   "bash",
			},
		},
	}

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: settings}
	require.NoError(t, runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()}))

	assert.Equal(t, `Tag:                        rev-42

OIDC
  AWS
    Role ARN:               arn:aws:iam::123:role/pulumi-deploy
    Session name:           pulumi-deploy
    Session duration:       30m0s
    Policy ARNs:            arn:aws:iam::aws:policy/ReadOnlyAccess
  GCP
    Project number:         123456
    Workload pool:          pulumi-pool
    Provider:               pulumi
    Service account:        pulumi@my-project.iam.gserviceaccount.com

Advanced
  Skip install dependencies: yes
  Shell:                    bash
`, buf.String())
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
