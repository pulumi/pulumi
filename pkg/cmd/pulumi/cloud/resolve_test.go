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

package cloud

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestAPIAgentLoginOnlyForLiveRequests(t *testing.T) {
	for _, mode := range []string{"live", "dry-run", "list", "describe", "human"} {
		t.Run(mode, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_HOME", t.TempDir())
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
			t.Setenv("PULUMI_CREDENTIALS_PATH", "")
			t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
			t.Setenv("PULUMI_ACCESS_TOKEN", "")
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
			for _, name := range agentdetect.DetectionEnvVars() {
				t.Setenv(name, "")
			}
			if mode != "human" {
				t.Setenv("AI_AGENT", "test-agent")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/orgs/agent-org/tokens":
					assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
					fmt.Fprint(w, `{"tokens":[]}`)
				case "/api/user/organizations/default":
					fmt.Fprint(w, `{"githubLogin":"agent-org"}`)
				case "/api/capabilities":
					fmt.Fprint(w, `{}`)
				default:
					http.Error(w, "unexpected API request", http.StatusBadRequest)
				}
			}))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			t.Setenv("PULUMI_API", server.URL)
			cache, err := specCachePath(server.URL)
			require.NoError(t, err)
			require.NoError(t, writeCachedSpec(cache, testSpecJSON))

			original := cmdBackend.DefaultLoginManager
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = original })
			cmdBackend.DefaultLoginManager = &cmdBackend.MockLoginManager{
				LoginF: func(ctx context.Context, _ pkgWorkspace.Context, sink diag.Sink, url string,
					project *workspace.Project, _, insecure bool, _ colors.Colorization,
				) (pkgBackend.Backend, error) {
					require.Equal(t, "live", mode, "only live agent requests may start login")
					require.Equal(t, server.URL, url)
					require.NoError(t, workspace.StoreAccount(url, workspace.Account{
						AccessToken: "test-token", Username: "agent-user", LastValidatedAt: time.Now(),
					}, false))
					return httpstate.New(ctx, sink, url, project, insecure)
				},
			}
			api := &apiCommand{envelopeVersion: SchemaVersion}
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			bindFlags(cmd, api)
			if mode != "live" {
				api.fields = []string{"orgName=agent-org"}
			}
			api.dryRun = mode == "dry-run"
			var out bytes.Buffer
			cmd.SetOut(&out)
			if mode == "list" || mode == "describe" {
				cmd = NewAPICmd()
				cmd.SetContext(t.Context())
				cmd.SetOut(&out)
				args := []string{mode}
				if mode == "describe" {
					args = append(args, "ListOrgTokens")
				}
				cmd.SetArgs(args)
				err = cmd.Execute()
			} else {
				err = runAPI(cmd, []string{"ListOrgTokens"}, api)
			}
			if mode == "human" {
				var apiErr *APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, ErrNotLoggedIn, apiErr.Envelope.Error.Code)
			} else {
				require.NoError(t, err)
				switch mode {
				case "live":
					assert.JSONEq(t, `{"tokens":[]}`, out.String())
				case "dry-run":
					assert.Contains(t, out.String(), server.URL+"/api/orgs/agent-org/tokens")
				default:
					assert.Contains(t, out.String(), "ListOrgTokens")
				}
			}
		})
	}
}

func TestResolveAnonymousContextUsesSelectedBackend(t *testing.T) {
	for _, source := range []string{"environment", "project", "credentials"} {
		t.Run(source, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_HOME", t.TempDir())
			t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
			t.Setenv("PULUMI_ACCESS_TOKEN", "")
			t.Setenv("PULUMI_BACKEND_URL", "")
			t.Setenv("PULUMI_API", "https://default.example.com")
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
			for _, name := range agentdetect.DetectionEnvVars() {
				t.Setenv(name, "")
			}
			const selected = "https://selected.example.com"
			switch source {
			case "environment":
				t.Setenv("PULUMI_BACKEND_URL", selected)
			case "project":
				require.NoError(t, os.WriteFile("Pulumi.yaml",
					[]byte("name: test\nruntime: nodejs\nbackend:\n  url: "+selected+"\n"), 0o600))
			case "credentials":
				require.NoError(t, workspace.StoreAccount(selected, workspace.Account{}, true))
			}
			require.NoError(t, workspace.SetBackendConfigDefaultOrg(selected, "configured-org"))
			resolved, err := ResolveContext(t.Context())
			require.NoError(t, err)
			assert.Equal(t, selected, resolved.CloudURL)
			assert.Equal(t, "configured-org", resolved.OrgName)
			assert.False(t, resolved.LoggedIn)
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "environment-org")
			resolved, err = ResolveContext(t.Context())
			require.NoError(t, err)
			assert.Equal(t, "environment-org", resolved.OrgName)
		})
	}
}

func TestResolveContextRejectsDIYWithoutCredentials(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_BACKEND_URL", "file://"+t.TempDir())
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	for _, name := range agentdetect.DetectionEnvVars() {
		t.Setenv(name, "")
	}
	_, err := ResolveContext(t.Context())
	require.ErrorContains(t, err, "requires the Pulumi Cloud backend")
}
