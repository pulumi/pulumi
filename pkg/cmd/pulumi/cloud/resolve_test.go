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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveContextBackendSelection(t *testing.T) {
	for _, test := range []struct {
		name          string
		apiOverride   bool
		authenticated bool
		savedHTTP     bool
		environment   string
		project       string
		wantError     bool
	}{
		{name: "saved DIY uses anonymous Cloud"},
		{name: "saved DIY uses API override", apiOverride: true},
		{name: "saved DIY uses stored API credentials", apiOverride: true, authenticated: true},
		{name: "saved HTTP uses stored credentials", savedHTTP: true, authenticated: true},
		{name: "environment DIY is rejected", environment: "file://~", apiOverride: true, wantError: true},
		{name: "project DIY is rejected", project: "file://~", apiOverride: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "api-org")
			previous := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = cmdBackend.NewLoginManager(nil)
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = previous })

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/user":
					assert.Equal(t, "token api-token", r.Header.Get("Authorization"))
					fmt.Fprint(w, `{"githubLogin":"api-user"}`)
				case "/api/capabilities":
					fmt.Fprint(w, `{"capabilities":[]}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			if test.apiOverride {
				t.Setenv("PULUMI_API", server.URL)
			}
			t.Setenv("PULUMI_BACKEND_URL", test.environment)
			if test.project != "" {
				project := "name: test-project\nruntime: nodejs\nbackend:\n  url: " + test.project + "\n"
				require.NoError(t, os.WriteFile("Pulumi.yaml", []byte(project), 0o600))
			}
			creds := workspace.Credentials{Current: "file://~", Accounts: map[string]workspace.Account{}}
			if test.savedHTTP {
				creds.Current = server.URL
			}
			if test.authenticated {
				creds.Accounts[server.URL] = workspace.Account{
					AccessToken: "api-token", Username: "api-user", LastValidatedAt: time.Now(),
				}
			}
			require.NoError(t, workspace.StoreCredentials(creds))

			resolved, err := ResolveContext(t.Context())
			if test.wantError {
				require.ErrorContains(t, err, "requires the Pulumi Cloud backend")
				return
			}
			require.NoError(t, err)
			expectedURL := client.PulumiCloudURL
			if test.apiOverride || test.savedHTTP {
				expectedURL = server.URL
			}
			assert.Equal(t, expectedURL, resolved.CloudURL)
			assert.Equal(t, test.authenticated, resolved.LoggedIn)
			if test.authenticated {
				user, _, _, err := resolved.Client.GetPulumiAccountDetails(t.Context())
				require.NoError(t, err)
				assert.Equal(t, "api-user", user)
				assert.Equal(t, "api-org", resolved.OrgName)
			}
			after, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, creds.Current, after.Current)
		})
	}
}
