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

package templates

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestRetrievePrivatePulumiCloudTemplateFallsBackToAgentCredentials(t *testing.T) {
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
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, "")
	t.Setenv("PULUMI_HOME", "")

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		gotAuth = req.Header.Get("Authorization")
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	cloudURL := "https://" + server.Listener.Addr().String()
	require.NoError(t, workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true))

	_, err = retrievePrivatePulumiCloudTemplate(server.URL + "/templates/private.zip")
	require.Error(t, err)
	assert.Equal(t, "token agent-token", gotAuth)
}
