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

package cmd

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates shared temporary agent credentials and os.Stderr
func TestProcessCmdErrorsPrintsAgentAuthRequiredInstruction(t *testing.T) {
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

	t.Setenv("CODEX_SANDBOX", "1")
	cloudURL := "https://api.example.com"
	expiresAt := time.Now().Add(2*24*time.Hour + 3*time.Hour + 9*time.Minute)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "locally-unexpired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(24 * time.Hour),
		CloudURL:   cloudURL,
	})
	require.NoError(t, err)

	output := captureStderr(t, func() {
		err = processCmdErrors(backenderr.LoginRequiredError{})
	})

	assert.True(t, result.IsBail(err))
	assert.Contains(t, output, "PULUMI_EPHEMERAL_AGENT_ACCOUNT_AUTH_REQUIRED")
	assert.Contains(t, output, "ACTION_REQUIRED=Tell the user to run pulumi login.")
	assert.Contains(t, output, "INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsKeepsGenericLoginRequiredOutsideAgentMode(t *testing.T) {
	t.Setenv("CODEX_SANDBOX", "")
	t.Setenv("AI_AGENT", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CURSOR_TRACE_ID", "")
	t.Setenv("CURSOR_AGENT", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "")

	err := backenderr.LoginRequiredError{}
	assert.Equal(t, err, processCmdErrors(err))
}

func captureStderr(t *testing.T, f func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	f()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	os.Stderr = oldStderr
	return string(output)
}
