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

// AI Generated - needs human review

import (
	"bytes"
	"context"
	"errors"
	"testing"

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

	assert.Equal(t, `Executor image:          pulumi/pulumi:latest
Working directory:       /work
Source repo URL:         https://github.com/acme/infra
Source branch:           main
Source commit:           abc123
Source repo dir:         stacks/prod
GitHub repository:       acme/infra
GitHub deploy commits:   true
GitHub preview PRs:      true
GitHub PR template:      false
GitHub paths:            1
Agent pool ID:           pool-1
Env var keys:            2
Pre-run commands:        1
`, buf.String())
}

func TestDeploymentSettingsGet_DefaultOutput_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: &apitype.DeploymentSettings{}}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetArgs{outputFormat: defaultDeploymentSettingsGetOutputFormat()})
	require.NoError(t, err)

	assert.Equal(t, `Executor image:          -
Working directory:       -
Source repo URL:         -
Source branch:           -
Source commit:           -
Source repo dir:         -
GitHub repository:       -
GitHub deploy commits:   false
GitHub preview PRs:      false
GitHub PR template:      false
GitHub paths:            0
Agent pool ID:           -
Env var keys:            0
Pre-run commands:        0
`, buf.String())
}

func TestDeploymentSettingsGet_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: sampleDeploymentSettings()}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"executorContext": {
			"workingDirectory": "/work",
			"executorImage": "pulumi/pulumi:latest"
		},
		"sourceContext": {
			"git": {
				"repoUrl": "https://github.com/acme/infra",
				"branch": "main",
				"commit": "abc123",
				"repoDir": "stacks/prod"
			}
		},
		"gitHub": {
			"repository": "acme/infra",
			"pullRequestTemplate": false,
			"deployCommits": true,
			"previewPullRequests": true,
			"paths": ["stacks/prod/**"]
		},
		"operationContext": {
			"preRunCommands": ["echo hi"],
			"operation": "update",
			"environmentVariables": {
				"FOO": "bar",
				"BAZ": "qux"
			}
		},
		"agentPoolID": "pool-1"
	}`, buf.String())
}

func TestDeploymentSettingsGet_JSONOutput_NilSlicesNormalized(t *testing.T) {
	t.Parallel()

	// GitHub set with nil Paths, Operation set with nil PreRunCommands and nil
	// EnvironmentVariables — JSON should expose them as empty arrays / object.
	settings := &apitype.DeploymentSettings{
		GitHub: &apitype.DeploymentSettingsGitHub{Repository: "acme/infra"},
		Operation: &apitype.OperationContext{
			Operation: apitype.Preview,
		},
	}

	var buf bytes.Buffer
	c := &mockDeploymentSettingsGetClient{resp: settings}
	err := runDeploymentSettingsGet(t.Context(), &buf, stubSettingsGetFactory(c),
		deploymentSettingsGetJSONArgs(t))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"gitHub": {
			"repository": "acme/infra",
			"pullRequestTemplate": false,
			"deployCommits": false,
			"previewPullRequests": false,
			"paths": []
		},
		"operationContext": {
			"preRunCommands": [],
			"operation": "preview",
			"environmentVariables": {}
		}
	}`, buf.String())
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
