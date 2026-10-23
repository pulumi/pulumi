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
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginCloudCredentialEnv(t *testing.T) {
	const cloudURL = "https://api.example.com"
	for _, test := range []struct {
		name, backendURL, envToken, storedToken, wantToken string
		agent                                              bool
	}{
		{
			name:        "environment precedes stored",
			backendURL:  cloudURL,
			envToken:    "env-token",
			storedToken: "stored-token",
			wantToken:   "env-token",
		},
		{name: "stored", backendURL: cloudURL, storedToken: "stored-token", wantToken: "stored-token"},
		{name: "agent", backendURL: cloudURL, storedToken: "agent-token", wantToken: "agent-token", agent: true},
		{name: "DIY", backendURL: "file:///tmp/state", envToken: "env-token"},
		{name: "logged out", backendURL: cloudURL},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_ACCESS_TOKEN", test.envToken)
			if test.storedToken != "" {
				account := workspace.Account{AccessToken: test.storedToken}
				if test.agent {
					t.Setenv("CODEX_SANDBOX", "1")
					t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
					require.NoError(t, workspace.StoreAgentAccount(test.backendURL, account, true))
				} else {
					require.NoError(t, workspace.StoreAccount(test.backendURL, account, false))
				}
			}
			project := &workspace.Project{Backend: &workspace.ProjectBackend{URL: test.backendURL}}
			if test.agent {
				project = nil // Select the backend from agent credentials as well as its token.
			}
			lm := &MockLoginManager{}
			credentials, err := PluginCloudCredentialEnv(t.Context(), &pkgWorkspace.MockContext{}, lm, project)
			require.NoError(t, err)
			if test.wantToken == "" {
				assert.Nil(t, credentials)
			} else {
				assert.Equal(t, map[string]string{
					"PULUMI_API": test.backendURL, "PULUMI_ACCESS_TOKEN": test.wantToken,
				}, credentials)
			}
		})
	}
}
