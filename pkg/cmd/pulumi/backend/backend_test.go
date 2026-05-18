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

package backend

import (
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates environment and shared temporary agent credentials
func TestGetCurrentCloudURLFallsBackToAgentCredentials(t *testing.T) {
	clearAIAgentEnv(t)
	oldAgentCreds, err := workspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, workspace.DeleteAgentCredentials())
		require.NoError(t, workspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, workspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	t.Setenv(env.BackendURL.Var().Name(), "")
	t.Setenv("CODEX_SANDBOX", "1")

	err = workspace.StoreAgentAccount("https://api.agent.example", workspace.Account{AccessToken: "token-value"}, true)
	require.NoError(t, err)

	ws := &pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	url, err := getCurrentCloudURL(ws, nil)
	require.NoError(t, err)
	assert.Equal(t, "https://api.agent.example", url)
}

//nolint:paralleltest // mutates environment
func TestGetCurrentCloudURLDoesNotFallbackWithExplicitPath(t *testing.T) {
	clearAIAgentEnv(t)
	t.Setenv(env.BackendURL.Var().Name(), "")
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, "/explicit/pulumi")
	t.Setenv("CODEX_SANDBOX", "1")

	ws := &pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	_, err := getCurrentCloudURL(ws, nil)
	require.ErrorIs(t, err, assert.AnError)
}

//nolint:paralleltest // mutates environment
func TestGetCurrentCloudURLReturnsDefaultCredentialErrorsOutsideAgents(t *testing.T) {
	clearAIAgentEnv(t)
	t.Setenv(env.BackendURL.Var().Name(), "")

	ws := &pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	_, err := getCurrentCloudURL(ws, nil)
	require.ErrorIs(t, err, assert.AnError)
}

func clearAIAgentEnv(t *testing.T) {
	t.Helper()

	t.Setenv(workspace.PulumiCredentialsPathEnvVar, "")
	t.Setenv(env.Home.Var().Name(), "")

	for _, name := range []string{
		"AI_AGENT",
		"CURSOR_TRACE_ID",
		"CURSOR_AGENT",
		"GEMINI_CLI",
		"CODEX_SANDBOX",
		"CODEX_CI",
		"CODEX_THREAD_ID",
		"ANTIGRAVITY_AGENT",
		"AUGMENT_AGENT",
		"OPENCODE",
		"OPENCODE_CALLER",
		"OPENCODE_CLIENT",
		"CLAUDE_CODE_IS_COWORK",
		"CLAUDECODE",
		"CLAUDE_CODE",
		"REPL_ID",
		"COPILOT_MODEL",
		"COPILOT_ALLOW_ALL",
		"COPILOT_GITHUB_TOKEN",
		"GOOSE_PROVIDER",
	} {
		t.Setenv(name, "")
	}
}
