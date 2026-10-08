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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestLoginManagerUsesReturnedCredentialsWhenPersistenceIsSkipped(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		dirs := ptesting.IsolateCredentials(t)
		t.Setenv("CODEX_SANDBOX", "1")
		t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
		t.Setenv("PULUMI_ACCESS_TOKEN", "user-token")
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "user-org")
		badHome := filepath.Join(dirs.Home, "not-a-directory")
		require.NoError(t, os.WriteFile(badHome, []byte("not a directory"), 0o600))
		t.Setenv("PULUMI_HOME", badHome)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token user-token", r.Header.Get("Authorization"))
			switch r.URL.Path {
			case "/api/user":
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"githubLogin": "user", "organizations": []map[string]string{},
				}))
			case "/api/capabilities":
				require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
			default:
				t.Errorf("unexpected request: %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(server.Close)
		require.NoError(t, workspace.StoreAgentAccount(server.URL, workspace.Account{
			AccessToken: "agent-token", Username: "agent-user",
		}, true))

		manager := &lm{}
		var be pkgBackend.Backend
		var err error
		if interactive {
			be, err = manager.Login(
				t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL, nil, false, false, colors.Never)
		} else {
			be, err = manager.Current(t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL, nil, false)
		}
		require.NoError(t, err)
		require.NotNil(t, be)
		be.(httpstate.Backend).Capabilities(t.Context())
		username, _, _, err := be.CurrentUser()
		require.NoError(t, err)
		assert.Equal(t, "user", username)
		agent, err := workspace.GetAgentAccount(server.URL)
		require.NoError(t, err)
		assert.Equal(t, "agent-token", agent.AccessToken)
	}
}
