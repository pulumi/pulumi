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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCredentialsLoginHelper(args []string) int {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}
	if err := os.WriteFile(args[0], data, 0o600); err != nil {
		return 1
	}
	fmt.Print(args[1])
	return 0
}

func loginHelperFlags(t *testing.T, response string) ([]string, string) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	log := filepath.Join(t.TempDir(), "request.json")
	flags := make([]string, 0, 10)
	flags = append(flags, "--credential-helper", executable)
	for _, arg := range []string{"--", "-credential-helper-test", log, response} {
		flags = append(flags, "--credential-helper-arg", arg)
	}
	return flags, log
}

//nolint:paralleltest // IsolateCredentials changes the process environment.
func TestPluginCredentialsUseProjectBackend(t *testing.T) {
	ptesting.IsolateCredentials(t)
	const response = `{"version":1,"accessToken":"helper-token"}`
	flags, log := loginHelperFlags(t, response)
	session := credentialhelper.NewSession(io.Discard)
	require.NoError(t, session.Configure(flags[1], []string{"--", "-credential-helper-test", log, response}))
	lm := cmdBackend.NewLoginManager(diagtest.LogSink(t), session)
	project := &workspace.Project{Backend: &workspace.ProjectBackend{URL: "https://api.example.com"}}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}
	cloudCredentialEnv, err := cmdBackend.PluginCloudCredentialEnv(t.Context(), ws, lm, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"PULUMI_API": project.Backend.URL, "PULUMI_ACCESS_TOKEN": "helper-token",
	}, cloudCredentialEnv)
	var req struct {
		SelectedBackendURL string `json:"selectedBackendUrl"`
	}
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &req))
	assert.Equal(t, project.Backend.URL, req.SelectedBackendURL)
}

func TestLoginCredentialHelperSelection(t *testing.T) {
	for _, test := range []struct {
		name, explicit, environment, project, expectedSelected string
		interactive                                            bool
	}{
		{name: "helper precedes saved current"},
		{name: "explicit URL", explicit: "https://explicit.example.com", expectedSelected: "https://explicit.example.com"},
		{
			name: "environment URL", environment: "https://environment.example.com",
			expectedSelected: "https://environment.example.com",
		},
		{name: "project URL", project: "https://project.example.com", expectedSelected: "https://project.example.com"},
		{name: "interactive account", interactive: true, expectedSelected: "https://saved.example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
			t.Setenv("PULUMI_BACKEND_URL", test.environment)
			require.NoError(t, workspace.StoreAccount(
				"https://saved.example.com", workspace.Account{AccessToken: "stored"}, true))
			response := `{"version":1,"backendUrl":"https://helper.example.com"}`
			expectedURL := "https://helper.example.com"
			if test.expectedSelected != "" {
				response = `{"version":1,"backendUrl":"` + test.expectedSelected + `"}`
				expectedURL = test.expectedSelected
			}
			flags, log := loginHelperFlags(t, response)
			ws := &pkgWorkspace.MockContext{ReadProjectF: func(string) (*workspace.Project, string, error) {
				return &workspace.Project{Backend: &workspace.ProjectBackend{URL: test.project}}, "", nil
			}}
			lm := &cmdBackend.MockLoginManager{LoginF: func(
				ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
				project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
				_ ...cmdBackend.LoginOption,
			) (pkgBackend.Backend, error) {
				assert.Equal(t, expectedURL, url)
				return &pkgBackend.MockBackend{
					URLF:  func() string { return url },
					NameF: func() string { return "test" },
					CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
						return "user", nil, nil, nil
					},
				}, nil
			}}
			chooserCalled := false
			login := loginCmd{
				interactive: true,
				chooseAccount: func(accounts map[string]workspace.Account, _ display.Options) (string, error) {
					chooserCalled = true
					assert.Contains(t, accounts, "https://saved.example.com")
					return "https://saved.example.com", nil
				},
			}
			cmd := login.newCommand(ws, lm, env.Global())
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if test.explicit != "" {
				flags = append(flags, test.explicit)
			}
			if test.interactive {
				flags = append(flags, "--interactive")
			}
			cmd.SetArgs(flags)
			require.NoError(t, cmd.Execute())
			assert.Equal(t, test.interactive, chooserCalled)
			raw, err := os.ReadFile(log)
			require.NoError(t, err)
			var req map[string]any
			require.NoError(t, json.Unmarshal(raw, &req))
			if test.expectedSelected == "" {
				assert.NotContains(t, req, "selectedBackendUrl")
			} else {
				assert.Equal(t, test.expectedSelected, req["selectedBackendUrl"])
			}
			creds, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			require.NotNil(t, creds.CredentialHelper)
			assert.True(t, filepath.IsAbs(creds.CredentialHelper.Path))
		})
	}
}

func TestLoginHelperHTTPAuthentication(t *testing.T) {
	for _, headerOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("headerOnly=%v", headerOnly), func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
			t.Setenv("PULUMI_ACCESS_TOKEN", "ambient-token")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expected := "token helper-token"
				if headerOnly {
					expected = ""
				}
				assert.Equal(t, expected, r.Header.Get("Authorization"))
				assert.Equal(t, "session", r.Header.Get("X-Session"))
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/user":
					w.Header().Set("X-Pulumi-Warning", "a service warning during helper login")
					fmt.Fprint(w, `{
						"githubLogin":"helper-user",
						"organizations":[{"githubLogin":"helper-org"}],
						"tokenInfo":{"name":"helper-token"}
					}`)
				case "/api/user/organizations/default":
					fmt.Fprint(w, `{"githubLogin":"org"}`)
				case "/api/capabilities":
					fmt.Fprint(w, `{"capabilities":[]}`)
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			token := `"accessToken":"helper-token",`
			if headerOnly {
				token = ""
			}
			response := `{"version":1,` + token + `"headers":{"X-Session":["session"]}}`
			flags, log := loginHelperFlags(t, response)
			flags = append(flags, server.URL)
			cmd := NewLoginCmd(
				pkgWorkspace.Instance,
				cmdBackend.NewLoginManager(diagtest.LogSink(t), credentialhelper.NewSession(io.Discard)), env.Global())
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(flags)
			require.NoError(t, cmd.Execute())
			_, err := os.Stat(log)
			require.NoError(t, err)
			creds, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, server.URL, creds.Current)
			require.Contains(t, creds.Accounts, server.URL)
			assert.Equal(t, workspace.Account{}, creds.Accounts[server.URL])
			require.NotNil(t, creds.CredentialHelper)
			storedAccount := workspace.Account{
				AccessToken: "stored-token", Username: "stored-user", Organizations: []string{"stored-org"},
				TokenInformation: &workspace.TokenInformation{Name: "stored-token"},
			}
			require.NoError(t, workspace.StoreAccount(server.URL, storedAccount, false))
			storedAccount, err = workspace.GetAccount(server.URL)
			require.NoError(t, err)
			// Opening the backend again must use the saved helper without saving its token.
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
			be, err := cmdBackend.CurrentBackend(
				cmd.Context(), pkgWorkspace.Instance,
				cmdBackend.NewLoginManager(diagtest.LogSink(t), credentialhelper.NewSession(io.Discard)), nil,
				display.Options{Color: colors.Never})
			require.NoError(t, err)
			user, orgs, tokenInfo, err := be.CurrentUser()
			require.NoError(t, err)
			assert.Equal(t, "helper-user", user)
			assert.Equal(t, []string{"helper-org"}, orgs)
			assert.Equal(t, &workspace.TokenInformation{Name: "helper-token"}, tokenInfo)
			be.(httpstate.Backend).Capabilities(cmd.Context())
			_, err = be.GetDefaultOrg(cmd.Context())
			require.NoError(t, err)
			stored, err := workspace.GetAccount(server.URL)
			require.NoError(t, err)
			assert.Equal(t, storedAccount, stored)
		})
	}
}

func TestLoginHelperTLSSetting(t *testing.T) {
	for _, test := range []struct {
		name     string
		insecure bool
		stored   *workspace.Account
	}{
		{name: "new insecure backend", insecure: true},
		{name: "enable insecure", insecure: true, stored: &workspace.Account{AccessToken: "stored-token"}},
		{name: "disable insecure", stored: &workspace.Account{AccessToken: "stored-token", Insecure: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "alice")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "token helper-token", r.Header.Get("Authorization"))
				if r.URL.Path == "/api/user" {
					fmt.Fprint(w, `{"githubLogin":"alice"}`)
				} else {
					fmt.Fprint(w, `{}`)
				}
			})
			server := httptest.NewUnstartedServer(handler)
			if test.insecure {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			expected := workspace.Account{Insecure: test.insecure}
			if test.stored != nil {
				require.NoError(t, workspace.StoreAccount(server.URL, *test.stored, false))
				expected = *test.stored
				expected.Insecure = test.insecure
			}
			flags, _ := loginHelperFlags(t, `{"version":1,"accessToken":"helper-token"}`)
			args := append([]string{server.URL}, flags...)
			if test.insecure {
				args = append(args, "--insecure")
			}
			cmd := NewLoginCmd(pkgWorkspace.Instance, cmdBackend.NewLoginManager(diagtest.LogSink(t), nil), env.Global())
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			creds, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, server.URL, creds.Current)
			require.Contains(t, creds.Accounts, server.URL)
			assert.Equal(t, expected, creds.Accounts[server.URL])
			be, err := cmdBackend.NonInteractiveCurrentBackend(t.Context(), pkgWorkspace.Instance,
				cmdBackend.NewLoginManager(diagtest.LogSink(t), nil), nil)
			require.NoError(t, err)
			assert.Equal(t, test.insecure, be.(httpstate.Backend).Client().Insecure())
		})
	}
}

func TestOIDCRejectsHelperBeforeExecution(t *testing.T) {
	ptesting.IsolateCredentials(t)
	flags, log := loginHelperFlags(t, `{}`)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", flags[1])
	var args []string
	for i := 2; i < len(flags); i += 2 {
		args = append(args, flags[i+1])
	}
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", string(raw))
	cmd := NewLoginCmd(
		pkgWorkspace.Instance,
		cmdBackend.NewLoginManager(diagtest.LogSink(t), credentialhelper.NewSession(io.Discard)), env.Global())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--oidc-token", "irrelevant-token"})
	require.ErrorContains(t, cmd.Execute(), "cannot be used with a credentials helper")
	_, err = os.Stat(log)
	require.True(t, os.IsNotExist(err))
}
