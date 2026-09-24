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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRegistryDoesNotCreateAgentAccount(t *testing.T) {
	for _, credentials := range []string{"none", "user", "agent"} {
		t.Run(credentials, func(t *testing.T) {
			t.Setenv("AI_AGENT", "codex")
			t.Setenv("PULUMI_ACCESS_TOKEN", "")
			t.Setenv("PULUMI_CREDENTIALS_PATH", "")
			t.Setenv("PULUMI_HOME", t.TempDir())
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
			t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")

			var signups, downloads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/agents/signup":
					signups.Add(1)
					w.WriteHeader(http.StatusBadRequest)
				case "/api/registry/packages/pulumi/pulumi/aws/versions/latest":
					downloads.Add(1)
					if credentials == "none" {
						assert.Empty(t, r.Header.Get("Authorization"))
					} else {
						assert.Equal(t, "token existing-token", r.Header.Get("Authorization"))
					}
					fmt.Fprint(w, `{"name":"aws","publisher":"pulumi","source":"pulumi","version":"1.0.0"}`)
				case "/api/user":
					fmt.Fprint(w, `{"githubLogin":"existing-user","organizations":[]}`)
				case "/api/capabilities":
					fmt.Fprint(w, `{}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_API", server.URL)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)

			account := workspace.Account{
				AccessToken: "existing-token", Username: "existing-user", LastValidatedAt: time.Now(),
			}
			switch credentials {
			case "user":
				require.NoError(t, workspace.StoreAccount(server.URL, account, true))
			case "agent":
				require.NoError(t, workspace.StoreAgentAccount(server.URL, account, true))
			}

			registry := NewDefaultRegistry(t.Context(), cmdBackend.DefaultLoginManager,
				pkgWorkspace.Instance, nil, diagtest.LogSink(t), env.Global())
			metadata, err := registry.GetPackage(t.Context(), "pulumi", "pulumi", "aws", nil)
			require.NoError(t, err)
			assert.Equal(t, "aws", metadata.Name)
			assert.EqualValues(t, 1, downloads.Load())
			assert.Zero(t, signups.Load())
		})
	}
}
