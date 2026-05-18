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
	"encoding/json"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedEditPatch struct {
	stack client.StackIdentifier
	patch json.RawMessage
}

type mockDeploymentSettingsEditClient struct {
	patchErr   error
	getResp    *apitype.DeploymentSettings
	getErr     error
	captured   *capturedEditPatch
	encryptErr error
}

func (m *mockDeploymentSettingsEditClient) PatchStackDeploymentSettings(
	_ context.Context, stack client.StackIdentifier, patch json.RawMessage,
) error {
	if m.captured != nil {
		m.captured.stack = stack
		m.captured.patch = patch
	}
	return m.patchErr
}

func (m *mockDeploymentSettingsEditClient) GetStackDeploymentSettings(
	_ context.Context, _ client.StackIdentifier,
) (*apitype.DeploymentSettings, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getResp, nil
}

func (m *mockDeploymentSettingsEditClient) EncryptStackDeploymentSettingsSecret(
	_ context.Context, _ client.StackIdentifier, secret string,
) (*apitype.SecretValue, error) {
	if m.encryptErr != nil {
		return nil, m.encryptErr
	}
	return &apitype.SecretValue{Ciphertext: "cipher:" + secret, Secret: true}, nil
}

func stubSettingsEditFactory(c deploymentSettingsEditClient) deploymentSettingsEditClientFactory {
	return func(_ context.Context, _ string) (deploymentSettingsEditClient, client.StackIdentifier, error) {
		return c, testStackID, nil
	}
}

func failingSettingsEditFactory(err error) deploymentSettingsEditClientFactory {
	return func(_ context.Context, _ string) (deploymentSettingsEditClient, client.StackIdentifier, error) {
		return nil, client.StackIdentifier{}, err
	}
}

// flagsSet builds the args.flagsChanged predicate from a list of flag names to simulate cobra having parsed those
// flags.
func flagsSet(names ...string) func(string) bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return func(name string) bool { return m[name] }
}

// branchArgs is the smallest valid args fixture: --branch=feature.
func branchArgs() deploymentSettingsEditArgs {
	return deploymentSettingsEditArgs{
		branch:       "feature",
		flagsChanged: flagsSet(flagBranch),
		outputFormat: defaultDeploymentSettingsGetOutputFormat(),
	}
}

func captureEditPatch(t *testing.T, args deploymentSettingsEditArgs, c *mockDeploymentSettingsEditClient) json.RawMessage {
	t.Helper()
	captured := &capturedEditPatch{}
	c.captured = captured
	if c.getResp == nil {
		c.getResp = &apitype.DeploymentSettings{}
	}
	args.outputFormat = defaultDeploymentSettingsGetOutputFormat()
	var buf bytes.Buffer
	require.NoError(t, runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c), args))
	require.NotNil(t, captured.patch)
	return captured.patch
}

func TestDeploymentSettingsEdit_DefaultOutput(t *testing.T) {
	t.Parallel()

	captured := &capturedEditPatch{}
	c := &mockDeploymentSettingsEditClient{
		getResp:  sampleDeploymentSettings(),
		captured: captured,
	}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c), branchArgs())
	require.NoError(t, err)

	assert.Equal(t, testStackID, captured.stack)
	require.NotNil(t, captured.patch)
	assert.JSONEq(t, `{"sourceContext":{"git":{"branch":"feature"}}}`, string(captured.patch))

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

func TestDeploymentSettingsEdit_JSONOutput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{getResp: sampleDeploymentSettings()}

	args := branchArgs()
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c), args)
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

func TestDeploymentSettingsEdit_NoInput(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to do")
}

func TestDeploymentSettingsEdit_PatchError(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{patchErr: errors.New("boom")}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c), branchArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "editing deployment settings")
	assert.Contains(t, err.Error(), "boom")
}

func TestDeploymentSettingsEdit_GetAfterPatchError(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{getErr: errors.New("get boom")}

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c), branchArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment settings")
	assert.Contains(t, err.Error(), "get boom")
}

func TestDeploymentSettingsEdit_FactoryError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		failingSettingsEditFactory(errors.New("not logged in")), branchArgs())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestDeploymentSettingsEdit_BranchFlag(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		branch:       "feature",
		flagsChanged: flagsSet(flagBranch),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{"sourceContext":{"git":{"branch":"feature"}}}`, string(got))
}

func TestDeploymentSettingsEdit_GitHubSourceFlags(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		githubRepo:   "acme/infra",
		branch:       "main",
		folder:       "stacks/prod",
		previewPRs:   true,
		pushToDeploy: true,
		pathFilters:  []string{"stacks/prod/**"},
		flagsChanged: flagsSet(flagGitHubRepo, flagBranch, flagFolder,
			flagPreviewPRs, flagPushToDeploy, flagPathFilter),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"gitHub": {
			"repository": "acme/infra",
			"previewPullRequests": true,
			"deployCommits": true,
			"paths": ["stacks/prod/**"]
		},
		"sourceContext": {"git": {"branch": "main", "repoDir": "stacks/prod"}}
	}`, string(got))
}

func TestDeploymentSettingsEdit_TristateFalse(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		previewPRs:   false,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{"gitHub":{"previewPullRequests":false}}`, string(got))
}

func TestDeploymentSettingsEdit_RunnerPoolEmptyClears(t *testing.T) {
	t.Parallel()
	// --runner-pool "" maps to JSON null so the server clears the field
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		runnerPool:   "",
		flagsChanged: flagsSet(flagRunnerPool),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{"agentPoolID":null}`, string(got))
}

func TestDeploymentSettingsEdit_ExecutorImageEmptyClears(t *testing.T) {
	t.Parallel()
	// --executor-image "" maps to JSON null so the server clears the field
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		executorImage: "",
		flagsChanged:  flagsSet(flagExecutorImage),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{"executorContext":{"executorImage":null}}`, string(got))
}

func TestDeploymentSettingsEdit_EnvVarsAndRemove(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		envVars:       []string{"FOO=bar", "BAZ=qux"},
		secretEnvVars: []string{"API_KEY=s3cret"},
		removeEnv:     []string{"STALE"},
		flagsChanged:  flagsSet(flagEnv, flagSecretEnv, flagRemoveEnv),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"environmentVariables": {
				"FOO": "bar",
				"BAZ": "qux",
				"API_KEY": {"ciphertext": "cipher:s3cret"},
				"STALE": null
			}
		}
	}`, string(got))
}

func TestDeploymentSettingsEdit_DuplicateEnvKey(t *testing.T) {
	t.Parallel()
	c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}
	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{
			envVars:      []string{"FOO=bar"},
			removeEnv:    []string{"FOO"},
			flagsChanged: flagsSet(flagEnv, flagRemoveEnv),
			outputFormat: defaultDeploymentSettingsGetOutputFormat(),
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FOO")
}

func TestDeploymentSettingsEdit_OIDCAWS(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		oidcAWSRoleARN:     "arn:aws:iam::123:role/pulumi-deploy",
		oidcAWSSessionName: "pulumi-deploy",
		oidcAWSDuration:    "30m",
		oidcAWSPolicyARNs:  []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
		flagsChanged: flagsSet(flagOIDCAWSRoleARN, flagOIDCAWSSessionName,
			flagOIDCAWSDuration, flagOIDCAWSPolicyARN),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"oidc": {
				"aws": {
					"roleArn": "arn:aws:iam::123:role/pulumi-deploy",
					"sessionName": "pulumi-deploy",
					"duration": "30m",
					"policyArns": ["arn:aws:iam::aws:policy/ReadOnlyAccess"]
				}
			}
		}
	}`, string(got))
}

// Setting a single OIDC field must only touch that field — relies on the
// server's deep merge to leave the rest of the AWS config alone.
func TestDeploymentSettingsEdit_OIDCPartialUpdate(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		oidcAWSRoleARN: "arn:aws:iam::123:role/pulumi-deploy",
		flagsChanged:   flagsSet(flagOIDCAWSRoleARN),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"oidc": {"aws": {"roleArn": "arn:aws:iam::123:role/pulumi-deploy"}}
		}
	}`, string(got))
}

func TestDeploymentSettingsEdit_OIDCClearFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
		want string
	}{
		{
			"aws",
			deploymentSettingsEditArgs{oidcAWSClear: true, flagsChanged: flagsSet(flagOIDCAWSClear)},
			`{"operationContext":{"oidc":{"aws":null}}}`,
		},
		{
			"azure",
			deploymentSettingsEditArgs{oidcAzureClear: true, flagsChanged: flagsSet(flagOIDCAzureClear)},
			`{"operationContext":{"oidc":{"azure":null}}}`,
		},
		{
			"gcp",
			deploymentSettingsEditArgs{oidcGCPClear: true, flagsChanged: flagsSet(flagOIDCGCPClear)},
			`{"operationContext":{"oidc":{"gcp":null}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, tc.args, &mockDeploymentSettingsEditClient{})
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestDeploymentSettingsEdit_OIDCClearConflictsWithSetters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
	}{
		{
			"aws-clear with aws-role-arn",
			deploymentSettingsEditArgs{
				oidcAWSClear:   true,
				oidcAWSRoleARN: "arn:aws:iam::123:role/x",
				flagsChanged:   flagsSet(flagOIDCAWSClear, flagOIDCAWSRoleARN),
			},
		},
		{
			"azure-clear with azure-client-id",
			deploymentSettingsEditArgs{
				oidcAzureClear:    true,
				oidcAzureClientID: "cid",
				flagsChanged:      flagsSet(flagOIDCAzureClear, flagOIDCAzureClientID),
			},
		},
		{
			"gcp-clear with gcp-project-number",
			deploymentSettingsEditArgs{
				oidcGCPClear:         true,
				oidcGCPProjectNumber: "123",
				flagsChanged:         flagsSet(flagOIDCGCPClear, flagOIDCGCPProjectNumber),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := tc.args
			args.outputFormat = defaultDeploymentSettingsGetOutputFormat()
			c := &mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}}
			var buf bytes.Buffer
			err := runDeploymentSettingsEdit(t.Context(), &buf,
				stubSettingsEditFactory(c), args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be combined")
		})
	}
}

func TestDeploymentSettingsEdit_OIDCAzure(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		oidcAzureClientID:       "11111111-1111-1111-1111-111111111111",
		oidcAzureTenantID:       "22222222-2222-2222-2222-222222222222",
		oidcAzureSubscriptionID: "33333333-3333-3333-3333-333333333333",
		flagsChanged: flagsSet(flagOIDCAzureClientID, flagOIDCAzureTenantID,
			flagOIDCAzureSubscriptionID),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"oidc": {
				"azure": {
					"clientId": "11111111-1111-1111-1111-111111111111",
					"tenantId": "22222222-2222-2222-2222-222222222222",
					"subscriptionId": "33333333-3333-3333-3333-333333333333"
				}
			}
		}
	}`, string(got))
}

func TestDeploymentSettingsEdit_OIDCGCP(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		oidcGCPProjectNumber:  "123456",
		oidcGCPWorkloadPoolID: "pulumi-pool",
		oidcGCPProviderID:     "pulumi",
		oidcGCPServiceAccount: "pulumi@my-project.iam.gserviceaccount.com",
		oidcGCPRegion:         "us-central1",
		oidcGCPTokenLifetime:  "1h",
		flagsChanged: flagsSet(flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID,
			flagOIDCGCPProviderID, flagOIDCGCPServiceAccount, flagOIDCGCPRegion,
			flagOIDCGCPTokenLifetime),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"oidc": {
				"gcp": {
					"projectId": "123456",
					"workloadPoolId": "pulumi-pool",
					"providerId": "pulumi",
					"serviceAccount": "pulumi@my-project.iam.gserviceaccount.com",
					"region": "us-central1",
					"tokenLifetime": "1h"
				}
			}
		}
	}`, string(got))
}

func TestDeploymentSettingsEdit_AdvancedToggles(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		skipInstallDeps: true,
		shell:           "bash",
		flagsChanged:    flagsSet(flagSkipInstallDeps, flagShell),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"options": {"skipInstallDependencies": true, "shell": "bash"}
		}
	}`, string(got))
}

func TestDeploymentSettingsEdit_EncryptError(t *testing.T) {
	t.Parallel()
	c := &mockDeploymentSettingsEditClient{
		getResp:    &apitype.DeploymentSettings{},
		encryptErr: errors.New("encrypt failed"),
	}
	var buf bytes.Buffer
	err := runDeploymentSettingsEdit(t.Context(), &buf,
		stubSettingsEditFactory(c),
		deploymentSettingsEditArgs{
			secretEnvVars: []string{"API=foo"},
			flagsChanged:  flagsSet(flagSecretEnv),
			outputFormat:  defaultDeploymentSettingsGetOutputFormat(),
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encrypting")
	assert.Contains(t, err.Error(), "encrypt failed")
}
