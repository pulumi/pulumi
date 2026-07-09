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
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	pulumi_workspace "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noCredsLoginManager int

// Current returns the current cloud backend if one is already logged in.
func (noCredsLoginManager) Current(
	ctx context.Context,
	cloudURL string,
	insecure, setCurrent bool,
) (*pulumi_workspace.Account, error) {
	return nil, nil
}

// Login logs into the target cloud URL and returns the cloud backend for it.
func (noCredsLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*pulumi_workspace.Account, error) {
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
) (*pulumi_workspace.Account, error) {
	return nil, errors.New("unauthorized")
}

func TestNoCreds(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
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
) (*pulumi_workspace.Account, error) {
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
) (*pulumi_workspace.Account, error) {
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
) (*pulumi_workspace.Account, error) {
	return nil, errors.New("not expected to call")
}

// Test for https://github.com/pulumi/esc/issues/367
func TestCurrentAccountButInvalidToken(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
	esc := &escCommand{
		command: "esc",
		ws: mockWorkspace(pulumi_workspace.Credentials{
			Current: "bobm",
			Accounts: map[string]pulumi_workspace.Account{
				"bobm": {
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
	accounts map[string]pulumi_workspace.Account
}

func (lm *provisioningLoginManager) Current(
	ctx context.Context, cloudURL string, insecure, setCurrent bool,
) (*pulumi_workspace.Account, error) {
	acct, ok := lm.accounts[cloudURL]
	if !ok {
		return nil, nil
	}
	return &acct, nil
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
) (*pulumi_workspace.Account, error) {
	if lm.accounts == nil {
		lm.accounts = map[string]pulumi_workspace.Account{}
	}
	acct := pulumi_workspace.Account{
		AccessToken: "agent-access-token",
		Username:    "agent-user",
		Insecure:    insecure,
	}
	lm.accounts[cloudURL] = acct
	return &acct, nil
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
) (*pulumi_workspace.Account, error) {
	return nil, errors.New("not expected to call")
}

func TestAgentModeUsesPulumiAPIAndLoginManagerWhenPulumiCredentialsUnreadable(t *testing.T) {
	t.Setenv("CODEX_SANDBOX", "seatbelt")
	t.Setenv("PULUMI_API", "http://localhost:8080")
	t.Setenv("PULUMI_HOME", "")
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")

	login := &provisioningLoginManager{}
	esc := &escCommand{
		command: "esc",
		login:   login,
		newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
			assert.Equal(t, "http://localhost:8080", backendURL)
			assert.Equal(t, "agent-access-token", accessToken)
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
		newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
			assert.Equal(t, "http://localhost:8081", backendURL)
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

func TestInvalidSelfHostedBackend(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
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

func TestFilestateBackend(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
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
		newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
			return client.New(userAgent, backendURL, accessToken, insecure)
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

func TestDefaultOrgConfiguration(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
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

	t.Run("prefers user configuration", func(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
		// GIVEN
		// The user has configured a default org (in an isolated PULUMI_HOME):
		userConfiguredDefaultOrg := "my-default-org"
		t.Setenv("PULUMI_HOME", t.TempDir())
		require.NoError(t, pulumi_workspace.SetBackendConfigDefaultOrg(backend, userConfiguredDefaultOrg))

		testClient := testPulumiClient{}
		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			ws:      mockWorkspace(creds),
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testClient
			},
		}

		// WHEN
		err := esc.getCachedClient(t.Context())

		// THEN
		require.NoError(t, err)
		assert.Equal(t, userConfiguredDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("falls back to backend client configuration", func(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
		// GIVEN
		// The user has not configured a default org (isolated, empty PULUMI_HOME):
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
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testClient
			},
		}

		// WHEN
		err := esc.getCachedClient(t.Context())

		// THEN
		require.NoError(t, err)
		assert.Equal(t, serviceDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("falls back to individual org as last resort", func(t *testing.T) { //nolint:paralleltest,lll // non-thread-safe shared state
		// GIVEN
		// The user has not configured a default org (isolated, empty PULUMI_HOME):
		t.Setenv("PULUMI_HOME", t.TempDir())

		// And the service has no opinion:
		testClient := testPulumiClient{defaultOrg: ""}

		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			ws:      mockWorkspace(creds),
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
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
