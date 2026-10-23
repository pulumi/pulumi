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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain also runs this test executable as a credential helper subprocess.
// Tests use os.Executable() with -credential-helper-test, a request log path, and a JSON response.
// It records the request from stdin and writes the response to stdout. Other invocations run tests normally.
func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg == "-credential-helper-test" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				os.Exit(1)
			}
			if err := os.WriteFile(os.Args[i+1], data, 0o600); err != nil {
				os.Exit(1)
			}
			fmt.Print(os.Args[i+2])
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

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
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "api-org")
			previous := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = cmdBackend.NewLoginManager(diagtest.LogSink(t), nil)
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

func TestResolveContextHelperBackendSelection(t *testing.T) {
	server := httptest.NewServer(resolveContextHandler(t, "token api-token"))
	defer server.Close()

	for _, test := range []struct {
		name        string
		current     string
		wantCurrent string
	}{
		{name: "preserves saved DIY backend", current: "file://~", wantCurrent: "file://~"},
		{name: "saves initial backend", current: "", wantCurrent: server.URL},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "api-org")
			previous := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = cmdBackend.NewLoginManager(diagtest.LogSink(t), nil)
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = previous })
			response := fmt.Sprintf(`{"version":1,"backendUrl":%q,"accessToken":"api-token"}`, server.URL)
			configureResolveContextHelper(t, response)
			require.NoError(t, workspace.StoreCredentials(workspace.Credentials{Current: test.current}))

			resolved, err := ResolveContext(t.Context())
			require.NoError(t, err)
			assert.Equal(t, server.URL, resolved.CloudURL)
			assert.True(t, resolved.LoggedIn)
			after, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, test.wantCurrent, after.Current)
		})
	}
}

func TestResolveContextInsecure(t *testing.T) {
	for _, credentials := range []string{"stored", "helper", "anonymous"} {
		t.Run(credentials, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "api-org")
			previous := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = cmdBackend.NewLoginManager(diagtest.LogSink(t), nil)
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = previous })

			expectedAuth := "token api-token"
			if credentials == "anonymous" {
				expectedAuth = ""
			}
			// The test server's certificate is untrusted, so requests fail unless the saved Insecure setting is honored.
			server := httptest.NewTLSServer(resolveContextHandler(t, expectedAuth))
			defer server.Close()
			account := workspace.Account{Insecure: true}
			switch credentials {
			case "stored":
				account.AccessToken = "api-token"
			case "helper":
				configureResolveContextHelper(t, `{"version":1,"accessToken":"api-token"}`)
			}
			require.NoError(t, workspace.StoreAccount(server.URL, account, true))

			resolved, err := ResolveContext(t.Context())
			require.NoError(t, err)
			assert.Equal(t, server.URL, resolved.CloudURL)
			assert.Equal(t, credentials != "anonymous", resolved.LoggedIn)
			// Exercise the returned client as well as credential validation during resolution.
			user, _, _, err := resolved.Client.GetPulumiAccountDetails(t.Context())
			require.NoError(t, err)
			assert.Equal(t, "api-user", user)
		})
	}
}

func resolveContextHandler(t *testing.T, authorization string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			assert.Equal(t, authorization, r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"githubLogin":"api-user"}`)
		case "/api/capabilities":
			fmt.Fprint(w, `{"capabilities":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func configureResolveContextHelper(t *testing.T, response string) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	args, err := json.Marshal([]string{
		"-credential-helper-test", filepath.Join(t.TempDir(), "request.json"), response,
	})
	require.NoError(t, err)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", executable)
	t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", string(args))
}
