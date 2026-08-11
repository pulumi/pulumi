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
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/spf13/pflag"

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
	patchErr error
	getResp  *apitype.DeploymentSettings
	getErr   error
	getCalls int
	captured *capturedEditPatch

	// notFoundUntilPatch models a stack that has never been configured: the service 404s the GET
	// until a PATCH creates the settings.
	notFoundUntilPatch bool
	patched            bool
}

func (m *mockDeploymentSettingsEditClient) PatchStackDeploymentSettings(
	_ context.Context, stack client.StackIdentifier, patch json.RawMessage,
) error {
	if m.captured != nil {
		m.captured.stack = stack
		m.captured.patch = patch
	}
	m.patched = true
	return m.patchErr
}

func (m *mockDeploymentSettingsEditClient) GetStackDeploymentSettings(
	_ context.Context, _ client.StackIdentifier,
) (*apitype.DeploymentSettings, error) {
	m.getCalls++
	if m.notFoundUntilPatch && !m.patched {
		return nil, &apitype.ErrorResponse{Code: http.StatusNotFound, Message: "not found"}
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getResp, nil
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

func captureEditPatch(
	t *testing.T, args deploymentSettingsEditArgs, c *mockDeploymentSettingsEditClient,
) json.RawMessage {
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
	assert.JSONEq(t, `{"sourceContext":{"git":{"branch":"feature","commit":null,"tag":null}}}`, string(captured.patch))

	assert.Equal(t, `Source: GitHub
  Repository:                    acme/infra
  Branch:                        main
  Commit:                        abc123
  Pulumi.yaml folder:            stacks/prod
  Run previews for PRs:          yes
  Run updates on push:           yes
  PR stack template:             no
  Path filters:                  stacks/prod/**

Deployment runner
  Runner pool:                   pool-1
  Executor image:                pulumi/pulumi:latest

Pre-run commands
  echo hi

Environment variables
  API_KEY:                       [secret]
  BAZ:                           qux
  FOO:                           bar
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
		"environmentVariables": [
			{"name": "API_KEY", "secret": true},
			{"name": "BAZ", "value": "qux"},
			{"name": "FOO", "value": "bar"}
		]
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
	assert.JSONEq(t, `{"sourceContext":{"git":{"branch":"feature","commit":null,"tag":null}}}`, string(got))
}

// The service rejects a merged source that carries both a branch and a commit, so setting one has to
// null the other. Clearing one must not null the other, or nothing is left to deploy from.
func TestDeploymentSettingsEdit_BranchAndCommitReplaceEachOther(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
		want string
	}{
		{
			"branch replaces commit",
			deploymentSettingsEditArgs{branch: "main", flagsChanged: flagsSet(flagBranch)},
			`{"sourceContext":{"git":{"branch":"main","commit":null,"tag":null}}}`,
		},
		{
			"commit replaces branch",
			deploymentSettingsEditArgs{commit: "abc123", flagsChanged: flagsSet(flagCommit)},
			`{"sourceContext":{"git":{"commit":"abc123","branch":null,"tag":null}}}`,
		},
		{
			"branch replaces a service-set tag",
			deploymentSettingsEditArgs{branch: "main", flagsChanged: flagsSet(flagBranch)},
			`{"sourceContext":{"git":{"branch":"main","commit":null,"tag":null}}}`,
		},
		{
			"empty branch clears only the branch",
			deploymentSettingsEditArgs{flagsChanged: flagsSet(flagBranch)},
			`{"sourceContext":{"git":{"branch":null}}}`,
		},
		{
			"empty commit clears only the commit",
			deploymentSettingsEditArgs{flagsChanged: flagsSet(flagCommit)},
			`{"sourceContext":{"git":{"commit":null}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, tc.args, &mockDeploymentSettingsEditClient{})
			assert.JSONEq(t, tc.want, string(got))
		})
	}
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
		"vcs": {
			"provider": "github",
			"repository": "acme/infra",
			"previewPullRequests": true,
			"deployCommits": true,
			"paths": ["stacks/prod/**"]
		},
		"sourceContext": {"git": {"branch": "main", "commit": null, "tag": null, "repoDir": "stacks/prod"}}
	}`, string(got))
}

func TestDeploymentSettingsEdit_TristateFalse(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		previewPRs:   false,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
			Provider:            apitype.VCSProviderGitHub,
			Repository:          "acme/infra",
			PreviewPullRequests: true,
		}),
	})
	assert.JSONEq(t, `{"vcs":{"provider":"github","repository":"acme/infra"}}`, string(got))
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

// A bare-string executorImage decodes server-side to an image whose credentials are explicitly
// null, wiping the stored registry credentials, so the patch carries an object with no credentials
// key at all.
func TestDeploymentSettingsEdit_ExecutorImageSendsObjectWithoutCredentials(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		executorImage: "acme/executor:1",
		flagsChanged:  flagsSet(flagExecutorImage),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{"executorContext":{"executorImage":{"reference":"acme/executor:1"}}}`, string(got))
	assert.NotContains(t, string(got), "credentials")
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
				"API_KEY": {"secret": "s3cret"},
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
			deploymentSettingsEditArgs{oidcAWSClear: true, flagsChanged: flagsSet(flagRemoveOIDCAWS)},
			`{"operationContext":{"oidc":{"aws":null}}}`,
		},
		{
			"azure",
			deploymentSettingsEditArgs{oidcAzureClear: true, flagsChanged: flagsSet(flagRemoveOIDCAzure)},
			`{"operationContext":{"oidc":{"azure":null}}}`,
		},
		{
			"gcp",
			deploymentSettingsEditArgs{oidcGCPClear: true, flagsChanged: flagsSet(flagRemoveOIDCGCP)},
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
				flagsChanged:   flagsSet(flagRemoveOIDCAWS, flagOIDCAWSRoleARN),
			},
		},
		{
			"azure-clear with azure-client-id",
			deploymentSettingsEditArgs{
				oidcAzureClear:    true,
				oidcAzureClientID: "cid",
				flagsChanged:      flagsSet(flagRemoveOIDCAzure, flagOIDCAzureClientID),
			},
		},
		{
			"gcp-clear with gcp-project-number",
			deploymentSettingsEditArgs{
				oidcGCPClear:         true,
				oidcGCPProjectNumber: "123",
				flagsChanged:         flagsSet(flagRemoveOIDCGCP, flagOIDCGCPProjectNumber),
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

// storedVCSSettings is what a GET returns for a stack whose source is already configured.
func storedVCSSettings(vcs apitype.DeploymentSettingsVCS) *apitype.DeploymentSettings {
	// A configured source always names a repository, and the patch carries the whole vcs object, so
	// default it rather than have every case that cares about other fields repeat it.
	if vcs.Repository == "" {
		vcs.Repository = "acme/infra"
	}
	return &apitype.DeploymentSettings{VCS: &vcs}
}

// runEditArgs drives runDeploymentSettingsEdit for the cases that assert on the error rather than
// on the emitted patch.
func runEditArgs(t *testing.T, args deploymentSettingsEditArgs, c *mockDeploymentSettingsEditClient) error {
	t.Helper()
	if c.getResp == nil {
		c.getResp = &apitype.DeploymentSettings{}
	}
	if c.captured == nil {
		c.captured = &capturedEditPatch{}
	}
	args.outputFormat = defaultDeploymentSettingsGetOutputFormat()
	var buf bytes.Buffer
	return runDeploymentSettingsEdit(t.Context(), &buf, stubSettingsEditFactory(c), args)
}

// runEditCmd exercises the cobra command itself, so flag registration, flag kinds and the
// mutual-exclusion rules are part of what is under test.
func runEditCmd(
	t *testing.T, c *mockDeploymentSettingsEditClient, argv ...string,
) (*capturedEditPatch, error) {
	t.Helper()
	captured := &capturedEditPatch{}
	c.captured = captured
	if c.getResp == nil {
		c.getResp = &apitype.DeploymentSettings{}
	}
	cmd := newDeploymentSettingsEditCmdWith(stubSettingsEditFactory(c))
	cmd.SetArgs(argv)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	return captured, cmd.ExecuteContext(t.Context())
}

func TestDeploymentSettingsEdit_VCSProviders(t *testing.T) {
	t.Parallel()
	for _, provider := range []apitype.VCSProvider{
		apitype.VCSProviderGitHub, apitype.VCSProviderGitLab, apitype.VCSProviderAzureDevOps,
		apitype.VCSProviderBitbucket, apitype.VCSProviderCustom,
	} {
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, deploymentSettingsEditArgs{
				repo:         "acme/infra",
				pushToDeploy: true,
				flagsChanged: flagsSet(flagRepo, flagPushToDeploy),
			}, &mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{Provider: provider}),
			})
			assert.JSONEq(t, `{"vcs":{
				"provider": "`+string(provider)+`",
				"repository": "acme/infra",
				"deployCommits": true
			}}`, string(got))
			assert.NotContains(t, string(got), "gitHub")
		})
	}
}

// The service replaces the vcs object wholesale, so editing one field has to resend the rest.
func TestDeploymentSettingsEdit_VCSEditPreservesStoredFields(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		previewPRs:   true,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
			Provider:       apitype.VCSProviderGitLab,
			Repository:     "acme/infra",
			Paths:          []string{"stacks/prod/**"},
			TagFilters:     []string{"v*"},
			InstallationID: "install-1",
			DeployTags:     true,
		}),
	})
	assert.JSONEq(t, `{"vcs":{
		"provider": "gitlab",
		"repository": "acme/infra",
		"paths": ["stacks/prod/**"],
		"tagFilters": ["v*"],
		"installationId": "install-1",
		"deployTags": true,
		"previewPullRequests": true
	}}`, string(got))
}

func TestDeploymentSettingsEdit_LegacyGitHubBlockBecomesVCS(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		previewPRs:   true,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{
		getResp: &apitype.DeploymentSettings{
			GitHub: &apitype.DeploymentSettingsGitHub{
				Repository:     "acme/infra",
				DeployCommits:  true,
				Paths:          []string{"stacks/prod/**"},
				InstallationID: "install-1",
			},
		},
	})
	assert.JSONEq(t, `{"vcs":{
		"provider": "github",
		"repository": "acme/infra",
		"deployCommits": true,
		"paths": ["stacks/prod/**"],
		"installationId": "install-1",
		"previewPullRequests": true
	}}`, string(got))
	assert.NotContains(t, string(got), "gitHub")
}

func TestDeploymentSettingsEdit_GitHubRepoStillWritesGitHubVCS(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		githubRepo:   "acme/infra",
		flagsChanged: flagsSet(flagGitHubRepo),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{"vcs":{"provider":"github","repository":"acme/infra"}}`, string(got))

	cmd := newDeploymentSettingsEditCmdWith(stubSettingsEditFactory(&mockDeploymentSettingsEditClient{}))
	assert.NotEmpty(t, cmd.Flags().Lookup(flagGitHubRepo).Deprecated)
}

func TestDeploymentSettingsEdit_GuardRejectsProviderChange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
	}{
		{
			"github-repo on a gitlab stack",
			deploymentSettingsEditArgs{
				githubRepo:   "acme/infra",
				flagsChanged: flagsSet(flagGitHubRepo),
			},
		},
		{
			"explicit vcs-provider on a gitlab stack",
			deploymentSettingsEditArgs{
				repo:         "acme/infra",
				vcsProvider:  "github",
				flagsChanged: flagsSet(flagRepo, flagVCSProvider),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
					Provider:   apitype.VCSProviderGitLab,
					Repository: "acme/infra",
				}),
			}
			err := runEditArgs(t, tc.args, c)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "gitlab")
			assert.Nil(t, c.captured.patch)
		})
	}
}

func TestDeploymentSettingsEdit_ReviewStackLabelsAreGitHubOnly(t *testing.T) {
	t.Parallel()

	args := deploymentSettingsEditArgs{
		reviewStackLabels: []string{"deploy"},
		flagsChanged:      flagsSet(flagReviewStackLabel),
	}

	got := captureEditPatch(t, args, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
			Provider:   apitype.VCSProviderGitHub,
			Repository: "acme/infra",
		}),
	})
	assert.JSONEq(t, `{"vcs":{
		"provider": "github",
		"repository": "acme/infra",
		"reviewStackLabels": ["deploy"]
	}}`, string(got))

	err := runEditArgs(t, args, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), flagReviewStackLabel)
}

func TestDeploymentSettingsEdit_VCSFlagNeedsAProvider(t *testing.T) {
	t.Parallel()
	err := runEditArgs(t, deploymentSettingsEditArgs{
		previewPRs:   true,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), flagVCSProvider)
}

func TestDeploymentSettingsEdit_UnknownVCSProvider(t *testing.T) {
	t.Parallel()
	err := runEditArgs(t, deploymentSettingsEditArgs{
		vcsProvider:  "svn",
		flagsChanged: flagsSet(flagVCSProvider),
	}, &mockDeploymentSettingsEditClient{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "svn")
}

// The conflict is with the merged object, so when only one of the two triggers was passed the
// message has to point at the stored setting instead of at a flag the user never typed.
func TestDeploymentSettingsEdit_DeployCommitsAndDeployTagsConflict(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		args   deploymentSettingsEditArgs
		stored apitype.DeploymentSettingsVCS
		want   string
	}{
		{
			"stored deploy commits",
			deploymentSettingsEditArgs{deployTags: true, flagsChanged: flagsSet(flagDeployTags)},
			apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab, DeployCommits: true},
			"this stack deploys on commits; pass --push-to-deploy=false to deploy on tags instead",
		},
		{
			"stored deploy tags",
			deploymentSettingsEditArgs{pushToDeploy: true, flagsChanged: flagsSet(flagPushToDeploy)},
			apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab, DeployTags: true},
			"this stack deploys on tags; pass --deploy-tags=false to deploy on commits instead",
		},
		{
			"both passed together",
			deploymentSettingsEditArgs{
				deployTags:   true,
				pushToDeploy: true,
				flagsChanged: flagsSet(flagDeployTags, flagPushToDeploy),
			},
			apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab},
			"--push-to-deploy and --deploy-tags are mutually exclusive",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := runEditArgs(t, tc.args, &mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(tc.stored),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// The service silently drops deployPullRequest when any standard trigger is on, so the merged object
// has to be refused rather than reported back as stored.
func TestDeploymentSettingsEdit_DeployPullRequestConflictsWithTriggers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		trigger string
		stored  apitype.DeploymentSettingsVCS
	}{
		{flagPushToDeploy, apitype.DeploymentSettingsVCS{
			Provider: apitype.VCSProviderGitHub, DeployCommits: true,
		}},
		{flagPreviewPRs, apitype.DeploymentSettingsVCS{
			Provider: apitype.VCSProviderGitHub, PreviewPullRequests: true,
		}},
		{flagPRTemplate, apitype.DeploymentSettingsVCS{
			Provider: apitype.VCSProviderGitHub, PullRequestTemplate: true,
		}},
	} {
		t.Run(tc.trigger, func(t *testing.T) {
			t.Parallel()
			err := runEditArgs(t, deploymentSettingsEditArgs{
				deployPullRequest: 42,
				flagsChanged:      flagsSet(flagDeployPullRequest),
			}, &mockDeploymentSettingsEditClient{getResp: storedVCSSettings(tc.stored)})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--"+flagDeployPullRequest+" is only honored")
		})
	}

	// Turning the trigger off in the same command is accepted: the check runs on the merged object.
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		deployPullRequest: 42,
		flagsChanged:      flagsSet(flagDeployPullRequest, flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
			Provider:            apitype.VCSProviderGitHub,
			PreviewPullRequests: true,
		}),
	})
	assert.JSONEq(t, `{"vcs":{"provider":"github","repository":"acme/infra","deployPullRequest":42}}`, string(got))
}

// A stack that already stores both triggers cannot be edited until one is turned off, but the
// message has to say that rather than name two flags the user never passed.
func TestDeploymentSettingsEdit_StoredBothTriggersNamesTheStoredState(t *testing.T) {
	t.Parallel()
	both := storedVCSSettings(apitype.DeploymentSettingsVCS{
		Provider:      apitype.VCSProviderGitHub,
		DeployCommits: true,
		DeployTags:    true,
	})

	err := runEditArgs(t, deploymentSettingsEditArgs{
		repo:         "acme/other",
		flagsChanged: flagsSet(flagRepo),
	}, &mockDeploymentSettingsEditClient{getResp: both})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stores both commit and tag triggers")

	// Turning one off in the same command resolves it.
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		flagsChanged: flagsSet(flagDeployTags),
	}, &mockDeploymentSettingsEditClient{getResp: both})
	assert.JSONEq(t,
		`{"vcs":{"provider":"github","repository":"acme/infra","deployCommits":true}}`, string(got))
}

// 0 is the documented way to clear a stored pull request number.
func TestDeploymentSettingsEdit_DeployPullRequestZeroClears(t *testing.T) {
	t.Parallel()
	pr := int64(42)
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		deployPullRequest: 0,
		flagsChanged:      flagsSet(flagDeployPullRequest),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
			Provider:          apitype.VCSProviderGitHub,
			DeployPullRequest: &pr,
		}),
	})
	assert.JSONEq(t, `{"vcs":{"provider":"github","repository":"acme/infra"}}`, string(got))
}

// A stored pull request number survives an edit that turns a trigger on. The service ignores it in
// that state, but deleting it here would lose a setting the user never mentioned.
func TestDeploymentSettingsEdit_EnablingATriggerKeepsAStoredDeployPullRequest(t *testing.T) {
	t.Parallel()
	pr := int64(42)
	for _, tc := range []struct {
		flag string
		args deploymentSettingsEditArgs
		want string
	}{
		{
			flagPreviewPRs,
			deploymentSettingsEditArgs{previewPRs: true, flagsChanged: flagsSet(flagPreviewPRs)},
			`{"vcs":{"provider":"github","repository":"acme/infra","previewPullRequests":true,"deployPullRequest":42}}`,
		},
		{
			flagPushToDeploy,
			deploymentSettingsEditArgs{pushToDeploy: true, flagsChanged: flagsSet(flagPushToDeploy)},
			`{"vcs":{"provider":"github","repository":"acme/infra","deployCommits":true,"deployPullRequest":42}}`,
		},
		{
			flagPRTemplate,
			deploymentSettingsEditArgs{prTemplate: true, flagsChanged: flagsSet(flagPRTemplate)},
			`{"vcs":{"provider":"github","repository":"acme/infra","pullRequestTemplate":true,"deployPullRequest":42}}`,
		},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, tc.args, &mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
					Provider:          apitype.VCSProviderGitHub,
					DeployPullRequest: &pr,
				}),
			})
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// PATCH is what creates the settings row, so a stack that has none must still be configurable.
func TestDeploymentSettingsEdit_CreatesSettingsWhenNoneStored(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{notFoundUntilPatch: true}
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		repo:         "acme/infra",
		vcsProvider:  string(apitype.VCSProviderGitHub),
		pushToDeploy: true,
		flagsChanged: flagsSet(flagRepo, flagVCSProvider, flagPushToDeploy),
	}, c)
	assert.JSONEq(t, `{"vcs":{
		"provider": "github",
		"repository": "acme/infra",
		"deployCommits": true
	}}`, string(got))
}

func TestDeploymentSettingsEdit_GetBeforePatchError(t *testing.T) {
	t.Parallel()

	err := runEditArgs(t, deploymentSettingsEditArgs{
		previewPRs:   true,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{getErr: errors.New("get boom")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading deployment settings")
}

// Only a 404 means "no settings yet"; every other status has to stay fatal.
func TestDeploymentSettingsEdit_GetBeforePatchNonNotFoundIsFatal(t *testing.T) {
	t.Parallel()

	err := runEditArgs(t, deploymentSettingsEditArgs{
		previewPRs:   true,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, &mockDeploymentSettingsEditClient{
		getErr: &apitype.ErrorResponse{Code: http.StatusInternalServerError, Message: "boom"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading deployment settings")
}

// Only the vcs flags pay for the extra round trip; sourceContext.git flags are provider-neutral.
func TestDeploymentSettingsEdit_GitFlagsDoNotReadSettingsFirst(t *testing.T) {
	t.Parallel()

	c := &mockDeploymentSettingsEditClient{}
	captureEditPatch(t, branchArgs(), c)
	assert.Equal(t, 1, c.getCalls)

	withVCS := &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab}),
	}
	captureEditPatch(t, deploymentSettingsEditArgs{
		previewPRs:   true,
		flagsChanged: flagsSet(flagPreviewPRs),
	}, withVCS)
	assert.Equal(t, 2, withVCS.getCalls)
}

// The vcs object replaces the stored one wholesale, so an empty repository would erase the source.
func TestDeploymentSettingsEdit_VCSNeedsARepository(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
		c    *mockDeploymentSettingsEditClient
	}{
		{
			"empty --repo on a configured stack",
			deploymentSettingsEditArgs{flagsChanged: flagsSet(flagRepo)},
			&mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
					Provider: apitype.VCSProviderGitLab,
				}),
			},
		},
		{
			"adopting a provider without naming a repository",
			deploymentSettingsEditArgs{
				vcsProvider:  string(apitype.VCSProviderGitLab),
				flagsChanged: flagsSet(flagVCSProvider),
			},
			&mockDeploymentSettingsEditClient{getResp: &apitype.DeploymentSettings{}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := runEditArgs(t, tc.args, tc.c)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "needs a repository")
		})
	}
}

func TestDeploymentSettingsEdit_VCSCoverageFlags(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		deployTags:        true,
		tagFilters:        []string{"v*", "release-*"},
		installationID:    "install-1",
		deployPullRequest: 42,
		flagsChanged: flagsSet(flagDeployTags, flagTagFilter, flagInstallationID,
			flagDeployPullRequest),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderBitbucket}),
	})
	assert.JSONEq(t, `{"vcs":{
		"provider": "bitbucket",
		"repository": "acme/infra",
		"deployTags": true,
		"tagFilters": ["v*", "release-*"],
		"installationId": "install-1",
		"deployPullRequest": 42
	}}`, string(got))
}

func TestDeploymentSettingsEdit_ClearPathFilters(t *testing.T) {
	t.Parallel()
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		pathFilters:  []string{""},
		flagsChanged: flagsSet(flagPathFilter),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
			Provider:   apitype.VCSProviderGitLab,
			Repository: "acme/infra",
			Paths:      []string{"stacks/prod/**"},
		}),
	})
	assert.JSONEq(t, `{"vcs":{"provider":"gitlab","repository":"acme/infra"}}`, string(got))
}

func TestDeploymentSettingsEdit_ClearVCSListFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
		want string
	}{
		{
			"tag filters",
			deploymentSettingsEditArgs{clearTagFilters: true, flagsChanged: flagsSet(flagClearTagFilters)},
			`{"vcs":{"provider":"github","repository":"acme/infra","reviewStackLabels":["deploy"]}}`,
		},
		{
			"review stack labels",
			deploymentSettingsEditArgs{
				clearReviewStackLabels: true,
				flagsChanged:           flagsSet(flagClearReviewStackLabel),
			},
			`{"vcs":{"provider":"github","repository":"acme/infra","tagFilters":["v*"]}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, tc.args, &mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
					Provider:          apitype.VCSProviderGitHub,
					Repository:        "acme/infra",
					TagFilters:        []string{"v*"},
					ReviewStackLabels: []string{"deploy"},
				}),
			})
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestDeploymentSettingsEdit_ClearReviewStackLabelsIsGitHubOnly(t *testing.T) {
	t.Parallel()
	err := runEditArgs(t, deploymentSettingsEditArgs{
		clearReviewStackLabels: true,
		flagsChanged:           flagsSet(flagClearReviewStackLabel),
	}, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), flagClearReviewStackLabel)
}

// Both list and map clears send null: an empty map is a no-op, because the server copies through
// every stored key the patch does not mention.
func TestDeploymentSettingsEdit_ClearListAndMapFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
		want string
	}{
		{
			"env",
			deploymentSettingsEditArgs{removeAllEnv: true, flagsChanged: flagsSet(flagRemoveAllEnv)},
			`{"operationContext":{"environmentVariables":null}}`,
		},
		{
			"pre-run commands",
			deploymentSettingsEditArgs{
				preRunCommands: []string{""},
				flagsChanged:   flagsSet(flagPreRunCommand),
			},
			`{"operationContext":{"preRunCommands":null}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, tc.args, &mockDeploymentSettingsEditClient{})
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestDeploymentSettingsEdit_ClearFlagsRejectFalse(t *testing.T) {
	t.Parallel()
	for _, flag := range presenceOnlyEditFlags {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			err := runEditArgs(t, deploymentSettingsEditArgs{flagsChanged: flagsSet(flag)},
				&mockDeploymentSettingsEditClient{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), flag)
		})
	}
}

// The clear has to survive the real flag parser: --pre-run-command "" must reach the patch builder
// as a one-element list rather than as an unset flag.
func TestDeploymentSettingsEdit_PreRunCommandEmptyStringClears(t *testing.T) {
	t.Parallel()
	captured, err := runEditCmd(t, &mockDeploymentSettingsEditClient{}, "--pre-run-command", "")
	require.NoError(t, err)
	assert.JSONEq(t, `{"operationContext":{"preRunCommands":null}}`, string(captured.patch))
}

func TestDeploymentSettingsEdit_ListFlagEmptyStringRejectsCompanions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ flag, value string }{
		{flagPreRunCommand, "echo hi"},
		{flagPathFilter, "stacks/prod/**"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			t.Parallel()
			_, err := runEditCmd(t, &mockDeploymentSettingsEditClient{},
				"--"+tc.flag, tc.value, "--"+tc.flag, "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be combined with other values")
		})
	}
}

// The superseded spellings shipped in v3.242.0, so they have to keep producing the same patch and
// keep telling the user what replaced them.
func TestDeploymentSettingsEdit_DeprecatedOIDCFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		deprecated string
		want       string
	}{
		{"oidc-aws-clear", `{"operationContext":{"oidc":{"aws":null}}}`},
		{"oidc-azure-clear", `{"operationContext":{"oidc":{"azure":null}}}`},
		{"oidc-gcp-clear", `{"operationContext":{"oidc":{"gcp":null}}}`},
	} {
		t.Run(tc.deprecated, func(t *testing.T) {
			t.Parallel()
			captured, err := runEditCmd(t, &mockDeploymentSettingsEditClient{}, "--"+tc.deprecated)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(captured.patch))
		})
	}
	for current, deprecated := range deprecatedEditFlags {
		t.Run(deprecated+" is marked deprecated", func(t *testing.T) {
			t.Parallel()
			cmd := newDeploymentSettingsEditCmdWith(
				stubSettingsEditFactory(&mockDeploymentSettingsEditClient{}))
			assert.Contains(t, cmd.Flags().Lookup(deprecated).Deprecated, current)
		})
	}
}

func TestDeploymentSettingsEdit_DurationFlagsClearWithNull(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args deploymentSettingsEditArgs
		want string
	}{
		{
			"aws duration",
			deploymentSettingsEditArgs{flagsChanged: flagsSet(flagOIDCAWSDuration)},
			`{"operationContext":{"oidc":{"aws":{"duration":null}}}}`,
		},
		{
			"gcp token lifetime",
			deploymentSettingsEditArgs{flagsChanged: flagsSet(flagOIDCGCPTokenLifetime)},
			`{"operationContext":{"oidc":{"gcp":{"tokenLifetime":null}}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := captureEditPatch(t, tc.args, &mockDeploymentSettingsEditClient{})
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestDeploymentSettingsEdit_DeployPullRequestRejectsNegative(t *testing.T) {
	t.Parallel()
	err := runEditArgs(t, deploymentSettingsEditArgs{
		deployPullRequest: -1,
		flagsChanged:      flagsSet(flagDeployPullRequest),
	}, &mockDeploymentSettingsEditClient{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), flagDeployPullRequest)
}

// A brace glob is one filter, not two: --path-filter is a repeatable string array rather than a
// comma-separated slice.
func TestDeploymentSettingsEdit_PathFilterKeepsBraceGlobs(t *testing.T) {
	t.Parallel()
	captured, err := runEditCmd(t, &mockDeploymentSettingsEditClient{
		getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{Provider: apitype.VCSProviderGitLab}),
	}, "--path-filter", "**/{apps,libs}/**")
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"vcs":{"provider":"gitlab","repository":"acme/infra","paths":["**/{apps,libs}/**"]}}`,
		string(captured.patch))
}

func TestDeploymentSettingsEdit_MutuallyExclusiveFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		argv []string
		want string
	}{
		{"branch and commit", []string{"--branch", "main", "--commit", "abc123"}, "[branch commit]"},
		{"env and remove-all-env", []string{"--env", "A=1", "--remove-all-env"}, "[env remove-all-env]"},
		{"secret env and remove-all-env", []string{"--secret-env", "A=1", "--remove-all-env"}, "[secret-env remove-all-env]"},
		{"remove env and remove-all-env", []string{"--remove-env", "A", "--remove-all-env"}, "[remove-env remove-all-env]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runEditCmd(t, &mockDeploymentSettingsEditClient{}, tc.argv...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// Driven through cobra so an explicit --flag=false really is parsed, which the args-level test
// cannot express. Passing the flag as true must be accepted: presenceOnlyFlagValue reads each clear flag
// out of the args struct by name, and a missing case there returns false, which would otherwise
// turn a valid clear into this same error.
func TestDeploymentSettingsEdit_ClearFlagsArePresenceOnly(t *testing.T) {
	t.Parallel()
	for _, flag := range presenceOnlyEditFlags {
		t.Run(flag+" rejects false", func(t *testing.T) {
			t.Parallel()
			_, err := runEditCmd(t, &mockDeploymentSettingsEditClient{}, "--"+flag+"=false")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "does not accept a false value")
		})
		t.Run(flag+" accepts true", func(t *testing.T) {
			t.Parallel()
			// A stored provider so that clear flags writing into vcs have one to resolve against.
			c := &mockDeploymentSettingsEditClient{
				getResp: storedVCSSettings(apitype.DeploymentSettingsVCS{
					Provider: apitype.VCSProviderGitHub,
				}),
			}
			_, err := runEditCmd(t, c, "--"+flag)
			require.NoError(t, err)
		})
	}
}

// Every flag the command registers has to appear in editFlagNames, or anyEditFlagSet reports
// "nothing to do" and the flag silently does nothing.
func TestDeploymentSettingsEdit_EditFlagNamesCoversEveryFlag(t *testing.T) {
	t.Parallel()

	// --stack and --output are wiring, not settings. A deprecated spelling is resolved to its
	// replacement by flagsChanged, so only the replacement belongs in the table.
	excluded := make([]string, 0, 2+len(deprecatedEditFlags))
	excluded = append(excluded, "stack", "output")
	for _, deprecated := range deprecatedEditFlags {
		excluded = append(excluded, deprecated)
	}

	cmd := newDeploymentSettingsEditCmdWith(stubSettingsEditFactory(&mockDeploymentSettingsEditClient{}))
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if slices.Contains(excluded, f.Name) {
			return
		}
		assert.Contains(t, editFlagNames, f.Name)
	})
}

func TestDeploymentSettingsEdit_SecretEnvWireForm(t *testing.T) {
	t.Parallel()
	// Secret env vars are sent in plaintext-secret wire form ({"secret": ...}); the server
	// encrypts them on PATCH.
	got := captureEditPatch(t, deploymentSettingsEditArgs{
		secretEnvVars: []string{"API=foo"},
		flagsChanged:  flagsSet(flagSecretEnv),
	}, &mockDeploymentSettingsEditClient{})
	assert.JSONEq(t, `{
		"operationContext": {
			"environmentVariables": {
				"API": {"secret": "foo"}
			}
		}
	}`, string(got))
}
