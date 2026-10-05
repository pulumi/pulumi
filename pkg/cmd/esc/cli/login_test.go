// Copyright 2023, Pulumi Corporation.
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

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	pulumi_workspace "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noCredsLoginManager int

// Current returns credentials for the current backend if one is already logged in.
func (noCredsLoginManager) Current(
	ctx context.Context,
	cloudURL string,
	insecure, setCurrent bool,
) (*auth.Credentials, error) {
	return nil, nil
}

// Login logs into the target cloud URL and returns its credentials.
func (noCredsLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*auth.Credentials, error) {
	return nil, errors.New("unauthorized")
}

func (noCredsLoginManager) LoginWithOIDCToken(
	ctx context.Context,
	sink diag.Sink,
	cloudURL string,
	insecure bool,
	oidcTokenSource string,
	organization string,
	scope string,
	expiration time.Duration,
	setCurrent bool,
) (*auth.Credentials, error) {
	return nil, errors.New("unauthorized")
}

func TestNoCreds(t *testing.T) {
	t.Parallel()
	esc := &escCommand{
		ws:    mockWorkspace(pulumi_workspace.Credentials{}),
		login: noCredsLoginManager(0),
	}
	err := esc.getCachedClient(t.Context())
	assert.ErrorContains(t, err, "could not determine current cloud")
}

type invalidatedCredsLoginManager int

func (invalidatedCredsLoginManager) Current(
	ctx context.Context,
	cloudURL string,
	insecure, setCurrent bool,
) (*auth.Credentials, error) {
	// nil, nil is a valid response to Current and will be returned by the httpstate backend when an account is current
	// but it's token has expired.
	return nil, nil
}

func (invalidatedCredsLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*auth.Credentials, error) {
	return nil, errors.New("not expected to call")
}

func (invalidatedCredsLoginManager) LoginWithOIDCToken(
	ctx context.Context,
	sink diag.Sink,
	cloudURL string,
	insecure bool,
	oidcTokenSource string,
	organization string,
	scope string,
	expiration time.Duration,
	setCurrent bool,
) (*auth.Credentials, error) {
	return nil, errors.New("not expected to call")
}

// Test for https://github.com/pulumi/esc/issues/367
func TestCurrentAccountButInvalidToken(t *testing.T) {
	t.Parallel()
	esc := &escCommand{
		command: "esc",
		ws: mockWorkspace(pulumi_workspace.Credentials{
			Current: "https://api.pulumi.com",
			Accounts: map[string]pulumi_workspace.Account{
				"https://api.pulumi.com": {
					AccessToken: "expired",
					Username:    "bobm",
				},
			},
		}),
		login: invalidatedCredsLoginManager(0),
	}
	err := esc.getCachedClient(t.Context())
	assert.ErrorContains(t, err, "no credentials, please run `esc login` to log in")
}

type provisioningLoginManager struct {
	accounts   map[string]pulumi_workspace.Account
	setCurrent bool
}

func (lm *provisioningLoginManager) Current(
	ctx context.Context, cloudURL string, insecure, setCurrent bool,
) (*auth.Credentials, error) {
	lm.setCurrent = setCurrent
	acct, ok := lm.accounts[cloudURL]
	if !ok {
		return nil, nil
	}
	return testCredentials(cloudURL, acct), nil
}

func (lm *provisioningLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*auth.Credentials, error) {
	lm.setCurrent = current
	if lm.accounts == nil {
		lm.accounts = map[string]pulumi_workspace.Account{}
	}
	acct := pulumi_workspace.Account{
		AccessToken: "agent-access-token",
		Username:    "agent-user",
		Insecure:    insecure,
	}
	lm.accounts[cloudURL] = acct
	return testCredentials(cloudURL, acct), nil
}

func (lm *provisioningLoginManager) LoginWithOIDCToken(
	ctx context.Context,
	sink diag.Sink,
	cloudURL string,
	insecure bool,
	oidcTokenSource string,
	organization string,
	scope string,
	expiration time.Duration,
	setCurrent bool,
) (*auth.Credentials, error) {
	return nil, errors.New("not expected to call")
}

func TestAgentModeUsesPulumiAPIAndLoginManagerWhenPulumiCredentialsUnreadable(t *testing.T) {
	t.Setenv("CODEX_SANDBOX", "seatbelt")
	t.Setenv("PULUMI_API", "http://localhost:8080")
	t.Setenv("PULUMI_HOME", "")
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	login := &provisioningLoginManager{}
	esc := &escCommand{
		command: "esc",
		login:   login,
		newClient: func(userAgent string, credentials auth.Credentials) client.Client {
			assert.Equal(t, "http://localhost:8080", credentials.BackendURL)
			assert.Equal(t, "agent-access-token", credentials.HTTPAuth.AccessToken())
			return &testPulumiClient{defaultOrg: "agent-org"}
		},
		ws: &pkgWorkspace.MockContext{
			GetStoredCredentialsF: func() (pulumi_workspace.Credentials, error) {
				return pulumi_workspace.Credentials{}, errors.New(
					"failed to create '/Users/example/.pulumi': operation not permitted")
			},
		},
	}

	err := esc.getCachedClient(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", esc.account.BackendURL)
	assert.Equal(t, "agent-org", esc.account.DefaultOrg)
	assert.Contains(t, login.accounts, "http://localhost:8080")
}

func TestPulumiBackendURLEnvOverridesPulumiAPI(t *testing.T) {
	t.Setenv("CODEX_SANDBOX", "seatbelt")
	t.Setenv("PULUMI_API", "http://localhost:8080")
	t.Setenv("PULUMI_BACKEND_URL", "http://localhost:8081")
	t.Setenv("PULUMI_HOME", "")
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")

	login := &provisioningLoginManager{}
	esc := &escCommand{
		command: "esc",
		login:   login,
		newClient: func(userAgent string, credentials auth.Credentials) client.Client {
			assert.Equal(t, "http://localhost:8081", credentials.BackendURL)
			return &testPulumiClient{defaultOrg: "agent-org"}
		},
		ws: &pkgWorkspace.MockContext{
			GetStoredCredentialsF: func() (pulumi_workspace.Credentials, error) {
				return pulumi_workspace.Credentials{}, errors.New(
					"failed to create '/Users/example/.pulumi': operation not permitted")
			},
		},
	}

	err := esc.getCachedClient(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8081", esc.account.BackendURL)
	assert.Contains(t, login.accounts, "http://localhost:8081")
}

func TestInvalidSelfHostedBackend(t *testing.T) {
	t.Parallel()
	esc := &escCommand{ws: mockWorkspace(pulumi_workspace.Credentials{
		Current: "http://pulumi.com",
		Accounts: map[string]pulumi_workspace.Account{
			"http://pulumi.com": {},
		},
	})}
	err := esc.getCachedClient(t.Context())
	assert.ErrorContains(t, err, "not a valid self-hosted backend")

	esc.command = "pulumi"
	err = esc.getCachedClient(t.Context())
	assert.ErrorContains(t, err, "pulumi login")
}

func TestFilestateBackend(t *testing.T) {
	t.Parallel()
	esc := &escCommand{ws: mockWorkspace(pulumi_workspace.Credentials{
		Current: "gs://foo",
		Accounts: map[string]pulumi_workspace.Account{
			"gs://foo": {},
		},
	})}
	err := esc.getCachedClient(t.Context())
	assert.ErrorContains(t, err, "does not support Pulumi ESC")
	assert.ErrorContains(t, err, "log into the Pulumi Cloud backend")
}

func TestEnvVarOverridesAccounts(t *testing.T) {
	creds := pulumi_workspace.Credentials{
		Current: "https://api.pulumi.com",
		Accounts: map[string]pulumi_workspace.Account{
			"https://api.pulumi.com": {
				Username:    "test-user",
				AccessToken: "access-token",
			},
			"http://api.moolumi.com": {
				Username:    "test-user",
				AccessToken: "access-token",
			},
		},
	}

	// Configure a default org for each backend (in an isolated PULUMI_HOME) so lookupDefaultOrg
	// resolves locally and doesn't make a client call.
	t.Setenv("PULUMI_HOME", t.TempDir())
	for url := range creds.Accounts {
		require.NoError(t, pulumi_workspace.SetBackendConfigDefaultOrg(url, "test-user-org"))
	}

	esc := &escCommand{
		command: "esc",
		login:   &testLoginManager{creds: creds},
		ws:      mockWorkspace(creds),
		newClient: func(userAgent string, credentials auth.Credentials) client.Client {
			return client.NewWithHTTPAuth(
				userAgent, credentials.BackendURL, credentials.Insecure, credentials.HTTPAuth)
		},
	}

	// Verify default
	err := esc.getCachedClient(t.Context())
	require.NoError(t, err)
	assert.Equal(t, esc.client.URL(), "https://api.pulumi.com")

	t.Setenv("PULUMI_BACKEND_URL", "http://api.moolumi.com")

	// Verify custom backend is used, as env var dictates
	err = esc.getCachedClient(t.Context())
	require.NoError(t, err)
	assert.Equal(t, esc.client.URL(), "http://api.moolumi.com")

	t.Setenv("PULUMI_BACKEND_URL", "")

	// Verify default returns once env var is unset
	err = esc.getCachedClient(t.Context())
	require.NoError(t, err)
	assert.Equal(t, esc.client.URL(), "https://api.pulumi.com")
}

func TestDefaultOrgConfiguration(t *testing.T) {
	username := "test-user"
	backend := "https://api.pulumi.com"
	creds := pulumi_workspace.Credentials{
		Current: backend,
		Accounts: map[string]pulumi_workspace.Account{
			backend: {
				Username:    username,
				AccessToken: "access-token",
			},
		},
	}

	t.Run("prefers environment variable", func(t *testing.T) {
		// The user has set PULUMI_DEFAULT_ORGANIZATION and configured a default org (in an isolated PULUMI_HOME):
		envDefaultOrg := "env-default-org"
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", envDefaultOrg)
		t.Setenv("PULUMI_HOME", t.TempDir())
		require.NoError(t, pulumi_workspace.SetBackendConfigDefaultOrg(backend, "my-default-org"))

		testClient := testPulumiClient{}
		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			ws:      mockWorkspace(creds),
			newClient: func(userAgent string, credentials auth.Credentials) client.Client {
				return &testClient
			},
		}

		err := esc.getCachedClient(t.Context())

		require.NoError(t, err)
		assert.Equal(t, envDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("prefers user configuration", func(t *testing.T) {
		// GIVEN
		// The user has configured a default org (in an isolated PULUMI_HOME):
		userConfiguredDefaultOrg := "my-default-org"
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
		t.Setenv("PULUMI_HOME", t.TempDir())
		require.NoError(t, pulumi_workspace.SetBackendConfigDefaultOrg(backend, userConfiguredDefaultOrg))

		testClient := testPulumiClient{}
		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			ws:      mockWorkspace(creds),
			newClient: func(userAgent string, credentials auth.Credentials) client.Client {
				return &testClient
			},
		}

		// WHEN
		err := esc.getCachedClient(t.Context())

		// THEN
		require.NoError(t, err)
		assert.Equal(t, userConfiguredDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("falls back to backend client configuration", func(t *testing.T) {
		// GIVEN
		// The user has not configured a default org (isolated, empty PULUMI_HOME):
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
		t.Setenv("PULUMI_HOME", t.TempDir())

		// But the backend has an opinion on the default org:
		serviceDefaultOrg := "service-default-org"
		testClient := testPulumiClient{
			defaultOrg: serviceDefaultOrg,
		}

		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			ws:      mockWorkspace(creds),
			newClient: func(userAgent string, credentials auth.Credentials) client.Client {
				return &testClient
			},
		}

		// WHEN
		err := esc.getCachedClient(t.Context())

		// THEN
		require.NoError(t, err)
		assert.Equal(t, serviceDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("falls back to individual org as last resort", func(t *testing.T) {
		// GIVEN
		// The user has not configured a default org (isolated, empty PULUMI_HOME):
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
		t.Setenv("PULUMI_HOME", t.TempDir())

		// And the service has no opinion:
		testClient := testPulumiClient{defaultOrg: ""}

		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			ws:      mockWorkspace(creds),
			newClient: func(userAgent string, credentials auth.Credentials) client.Client {
				return &testClient
			},
		}

		// WHEN
		err := esc.getCachedClient(t.Context())

		// THEN
		require.NoError(t, err)
		assert.Equal(t, username, esc.account.DefaultOrg)
	})
}

func TestCredentialHelperSelection(t *testing.T) {
	for _, tt := range []struct {
		name           string
		projectBackend string
		wantSelected   string
		wantBackend    string
		helperURL      string
		setCurrent     bool
	}{
		{
			name: "nothing selected", helperURL: "https://helper.example.com",
			wantBackend: "https://helper.example.com", setCurrent: true,
		},
		{name: "declines", wantBackend: "https://api.pulumi.com"},
		{name: "selects fallback", helperURL: "https://api.pulumi.com", wantBackend: "https://api.pulumi.com"},
		{
			name: "project backend", projectBackend: "s3://project-state",
			wantSelected: "https://api.pulumi.com", wantBackend: "https://api.pulumi.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_API", "")
			var requests []credentialhelper.Request
			session := auth.NewSessionWithHelperFunc(func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				requests = append(requests, request)
				if request.SelectedBackendURL != "" {
					return nil, nil
				}
				return &credentialhelper.Response{BackendURL: tt.helperURL}, nil
			})
			login := &provisioningLoginManager{}
			esc := &escCommand{
				command: "esc",
				login:   login,
				session: session,
				newClient: func(userAgent string, credentials auth.Credentials) client.Client {
					return &testPulumiClient{}
				},
				ws: &pkgWorkspace.MockContext{
					ReadProjectF: func(string) (*pulumi_workspace.Project, string, error) {
						project := &pulumi_workspace.Project{}
						if tt.projectBackend != "" {
							project.Backend = &pulumi_workspace.ProjectBackend{URL: tt.projectBackend}
						}
						return project, "", nil
					},
				},
			}

			require.NoError(t, esc.getCachedClient(t.Context()))
			require.Len(t, requests, 1)
			assert.Equal(t, tt.wantSelected, requests[0].SelectedBackendURL)
			assert.Equal(t, tt.wantBackend, esc.account.BackendURL)
			assert.Equal(t, tt.setCurrent, login.setCurrent)
		})
	}
}

func TestDefaultOrgUsesAuthenticatedIdentity(t *testing.T) {
	for _, stored := range []string{"none", "account", "unreadable"} {
		t.Run(stored, func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "token helper-token", r.Header.Get("Authorization"))
				assert.Equal(t, "helper-header", r.Header.Get("X-Test-Auth"))
				_, err := io.WriteString(w, `{"githubLogin":"helper-user"}`)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			session := auth.NewSessionWithHelperFunc(func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				return &credentialhelper.Response{
					BackendURL:  request.SelectedBackendURL,
					AccessToken: "helper-token",
					Headers:     http.Header{"X-Test-Auth": {"helper-header"}},
				}, nil
			})
			creds := pulumi_workspace.Credentials{Current: server.URL}
			if stored == "account" {
				creds.Accounts = map[string]pulumi_workspace.Account{
					server.URL: {AccessToken: "stored-token", Username: "stored-user"},
				}
			}
			if stored == "unreadable" {
				require.NoError(t, os.WriteFile(filepath.Join(dirs.Home, "credentials.json"), []byte("invalid JSON"), 0o600))
			} else {
				require.NoError(t, pulumi_workspace.StoreCredentials(creds))
			}
			testClient := &testPulumiClient{}
			esc := newESC(&Options{
				Session: session,
				newClient: func(_ string, credentials auth.Credentials) client.Client {
					assert.Equal(t, server.URL, credentials.BackendURL)
					assert.Equal(t, "helper-user", credentials.Identity.Username)
					assert.Equal(t, "helper-token", credentials.HTTPAuth.AccessToken())
					assert.Same(t, session.HelperHTTPAuth(server.URL), credentials.HTTPAuth)
					return testClient
				},
			})

			require.NoError(t, esc.getCachedClient(t.Context()))
			assert.Same(t, testClient, esc.client)
			assert.Equal(t, "helper-user", esc.account.DefaultOrg)
		})
	}
}

func TestHelperHeadersExcludeStoredOAuth(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(fmt.Sprintf("accepted=%v", accepted), func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_ACCESS_TOKEN", "environment-token")
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "helper-user")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/user", r.URL.Path, "stored OAuth must not be refreshed")
				assert.Empty(t, r.Header.Get("Authorization"))
				if !accepted || r.Header.Get("X-Helper") != "refreshed" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, err := io.WriteString(w, `{"githubLogin":"helper-user"}`)
				require.NoError(t, err)
			}))
			defer server.Close()
			account := pulumi_workspace.Account{AccessToken: "stored-token", RefreshToken: "stored-refresh-token"}
			require.NoError(t, pulumi_workspace.StoreAccount(server.URL, account, true))
			var reasons []credentialhelper.Reason
			session := auth.NewSessionWithHelperFunc(func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				reasons = append(reasons, request.Reason)
				header := "initial"
				if request.Reason == credentialhelper.Rejected {
					header = "refreshed"
				}
				return &credentialhelper.Response{Headers: http.Header{"X-Helper": {header}}}, nil
			})
			esc := newESC(&Options{Session: session})
			err := esc.getCachedClient(t.Context())
			if accepted {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			assert.Equal(t, []credentialhelper.Reason{credentialhelper.Initial, credentialhelper.Rejected}, reasons)
			stored, err := pulumi_workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, account, stored.Accounts[server.URL])
		})
	}
}
