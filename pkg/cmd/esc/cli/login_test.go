// Copyright 2023, Pulumi Corporation.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/workspace"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	pulumi_workspace "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

type noCredsLoginManager int

// Current returns the current cloud backend if one is already logged in.
func (noCredsLoginManager) Current(ctx context.Context, cloudURL string, insecure, setCurrent bool) (*pulumi_workspace.Account, error) {
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

func TestNoCreds(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{
		workspace: workspace.New(fs, &testPulumiWorkspace{}),
		login:     noCredsLoginManager(0),
	}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "could not determine current cloud")
}

type invalidatedCredsLoginManager int

func (invalidatedCredsLoginManager) Current(ctx context.Context, cloudURL string, insecure, setCurrent bool) (*pulumi_workspace.Account, error) {
	// nil, nil is a valid response to Current and will be returned by the httpstate backend when an account is current but it's token has expired.
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
	return nil, fmt.Errorf("not expected to call")
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
	return nil, fmt.Errorf("not expected to call")
}

// Test for https://github.com/pulumi/esc/issues/367
func TestCurrentAccountButInvalidToken(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{
		command: "esc",
		workspace: workspace.New(fs, &testPulumiWorkspace{
			credentials: pulumi_workspace.Credentials{
				Current: "bobm",
				Accounts: map[string]pulumi_workspace.Account{
					"bobm": {
						AccessToken: "expired",
						Username:    "bobm",
					},
				},
			},
		}),
		login: invalidatedCredsLoginManager(0),
	}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "no credentials. Please run `esc login` to log in.")
}

func TestInvalidSelfHostedBackend(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{workspace: workspace.New(fs, &testPulumiWorkspace{
		credentials: pulumi_workspace.Credentials{
			Current: "http://pulumi.com",
			Accounts: map[string]pulumi_workspace.Account{
				"http://pulumi.com": {},
			},
		},
	})}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "not a valid self-hosted backend")

	esc.command = "pulumi"
	err = esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "pulumi login")
}

func TestFilestateBackend(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{workspace: workspace.New(fs, &testPulumiWorkspace{
		credentials: pulumi_workspace.Credentials{
			Current: "gs://foo",
			Accounts: map[string]pulumi_workspace.Account{
				"gs://foo": {},
			},
		},
	})}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "does not support Pulumi ESC")
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

	// Configure a default org to skip mocking a client call for default org
	backendConfig := make(map[string]pulumi_workspace.BackendConfig, len(creds.Accounts))
	for url := range creds.Accounts {
		backendConfig[url] = pulumi_workspace.BackendConfig{DefaultOrg: "test-user-org"}
	}

	esc := &escCommand{
		command: "esc",
		login:   &testLoginManager{creds: creds},
		newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
			return client.New(userAgent, backendURL, accessToken, insecure)
		},
		workspace: workspace.New(testFS{}, &testPulumiWorkspace{
			config: pulumi_workspace.PulumiConfig{BackendConfig: backendConfig}}),
	}

	// Verify default
	err := esc.getCachedClient(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, esc.client.URL(), "https://api.pulumi.com")

	t.Setenv("PULUMI_BACKEND_URL", "http://api.moolumi.com")

	// Verify custom backend is used, as env var dictates
	err = esc.getCachedClient(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, esc.client.URL(), "http://api.moolumi.com")

	t.Setenv("PULUMI_BACKEND_URL", "")

	// Verify default returns once env var is unset
	err = esc.getCachedClient(context.Background())
	assert.NoError(t, err)
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

	t.Run("prefers user configuration", func(t *testing.T) {
		// GIVEN
		// The user has configured a default org:
		userConfiguredDefaultOrg := "my-default-org"
		testWorkspace := workspace.New(testFS{}, &testPulumiWorkspace{
			config: pulumi_workspace.PulumiConfig{
				BackendConfig: map[string]pulumi_workspace.BackendConfig{
					backend: {
						DefaultOrg: userConfiguredDefaultOrg,
					},
				},
			},
		})

		testClient := testPulumiClient{}
		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testClient
			},
			workspace: testWorkspace,
		}

		// WHEN
		err := esc.getCachedClient(context.Background())

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, userConfiguredDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("falls back to backend client configuration", func(t *testing.T) {
		// GIVEN
		// The user has not configured a default org:
		testWorkspace := workspace.New(testFS{}, &testPulumiWorkspace{})

		// But the backend has an opinion on the default org:
		serviceDefaultOrg := "service-default-org"
		testClient := testPulumiClient{
			defaultOrg: serviceDefaultOrg,
		}

		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testClient
			},
			workspace: testWorkspace,
		}

		// WHEN
		err := esc.getCachedClient(context.Background())

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, serviceDefaultOrg, esc.account.DefaultOrg)
	})

	t.Run("falls back to individual org as last resort", func(t *testing.T) {
		// GIVEN
		// The user has not configured a default org:
		testWorkspace := workspace.New(testFS{}, &testPulumiWorkspace{})

		// And the service has no opinion:
		testClient := testPulumiClient{defaultOrg: ""}

		esc := &escCommand{
			command: "esc",
			login:   &testLoginManager{creds: creds},
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testClient
			},
			workspace: testWorkspace,
		}

		// WHEN
		err := esc.getCachedClient(context.Background())

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, username, esc.account.DefaultOrg)
	})
}

type oidcLoginManager struct {
	creds                     pulumi_workspace.Credentials
	capturedOIDCTokenSource   string
	capturedOrganization      string
	capturedScope             string
	capturedExpiration        time.Duration
	loginWithOIDCTokenError   error
	loginWithOIDCTokenAccount *pulumi_workspace.Account
}

func (lm *oidcLoginManager) Current(ctx context.Context, cloudURL string, insecure, setCurrent bool) (*pulumi_workspace.Account, error) {
	if lm.creds.Current == "" {
		return nil, nil
	}
	acct, ok := lm.creds.Accounts[lm.creds.Current]
	if !ok {
		return nil, errors.New("unauthorized")
	}
	return &acct, nil
}

func (lm *oidcLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*pulumi_workspace.Account, error) {
	return nil, errors.New("should not be called when using OIDC token")
}

func (lm *oidcLoginManager) LoginWithOIDCToken(
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
	lm.capturedOIDCTokenSource = oidcTokenSource
	lm.capturedOrganization = organization
	lm.capturedScope = scope
	lm.capturedExpiration = expiration

	if lm.loginWithOIDCTokenError != nil {
		return nil, lm.loginWithOIDCTokenError
	}

	if lm.loginWithOIDCTokenAccount != nil {
		return lm.loginWithOIDCTokenAccount, nil
	}

	acct := pulumi_workspace.Account{
		Username:    "oidc-user",
		AccessToken: "oidc-access-token",
	}
	return &acct, nil
}

func TestOIDCTokenExchange(t *testing.T) {
	backend := "https://api.pulumi.com"
	creds := pulumi_workspace.Credentials{
		Current: backend,
		Accounts: map[string]pulumi_workspace.Account{
			backend: {
				Username:    "test-user",
				AccessToken: "access-token",
			},
		},
	}

	t.Run("successful OIDC login with organization token", func(t *testing.T) {
		// GIVEN
		loginMgr := &oidcLoginManager{creds: creds}
		fs := testFS{MapFS: make(map[string]*fstest.MapFile)}
		testWorkspace := workspace.New(fs, &testPulumiWorkspace{credentials: creds})

		var stdout, stderr bytes.Buffer
		cmd := newLoginCmd(&escCommand{
			command:   "esc",
			login:     loginMgr,
			workspace: testWorkspace,
			environ:   testEnviron{},
			stdout:    &stdout,
			stderr:    &stderr,
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testPulumiClient{user: "oidc-user"}
			},
		})

		// WHEN
		cmd.SetArgs([]string{
			"--oidc-token", "test-oidc-token",
			"--oidc-org", "my-org",
			"--oidc-expiration", "1h",
		})
		err := cmd.Execute()

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, "test-oidc-token", loginMgr.capturedOIDCTokenSource)
		assert.Equal(t, "my-org", loginMgr.capturedOrganization)
		assert.Equal(t, "", loginMgr.capturedScope)
		assert.Equal(t, 1*time.Hour, loginMgr.capturedExpiration)
	})

	t.Run("successful OIDC login with team token", func(t *testing.T) {
		// GIVEN
		loginMgr := &oidcLoginManager{creds: creds}
		fs := testFS{MapFS: make(map[string]*fstest.MapFile)}
		testWorkspace := workspace.New(fs, &testPulumiWorkspace{credentials: creds})

		var stdout, stderr bytes.Buffer
		cmd := newLoginCmd(&escCommand{
			command:   "esc",
			login:     loginMgr,
			workspace: testWorkspace,
			environ:   testEnviron{},
			stdout:    &stdout,
			stderr:    &stderr,
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testPulumiClient{user: "oidc-user"}
			},
		})

		// WHEN
		cmd.SetArgs([]string{
			"--oidc-token", "test-oidc-token",
			"--oidc-org", "my-org",
			"--oidc-team", "my-team",
		})
		err := cmd.Execute()

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, "test-oidc-token", loginMgr.capturedOIDCTokenSource)
		assert.Equal(t, "my-org", loginMgr.capturedOrganization)
		assert.Equal(t, "team:my-team", loginMgr.capturedScope)
		assert.Equal(t, 2*time.Hour, loginMgr.capturedExpiration) // Default expiration is 2h
	})

	t.Run("successful OIDC login with user token", func(t *testing.T) {
		// GIVEN
		loginMgr := &oidcLoginManager{creds: creds}
		fs := testFS{MapFS: make(map[string]*fstest.MapFile)}
		testWorkspace := workspace.New(fs, &testPulumiWorkspace{credentials: creds})

		var stdout, stderr bytes.Buffer
		cmd := newLoginCmd(&escCommand{
			command:   "esc",
			login:     loginMgr,
			workspace: testWorkspace,
			environ:   testEnviron{},
			stdout:    &stdout,
			stderr:    &stderr,
			newClient: func(userAgent, backendURL, accessToken string, insecure bool) client.Client {
				return &testPulumiClient{user: "oidc-user"}
			},
		})

		// WHEN
		cmd.SetArgs([]string{
			"--oidc-token", "test-oidc-token",
			"--oidc-org", "my-org",
			"--oidc-user", "my-user",
		})
		err := cmd.Execute()

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, "test-oidc-token", loginMgr.capturedOIDCTokenSource)
		assert.Equal(t, "my-org", loginMgr.capturedOrganization)
		assert.Equal(t, "user:my-user", loginMgr.capturedScope)
		assert.Equal(t, 2*time.Hour, loginMgr.capturedExpiration) // Default expiration is 2h
	})

	t.Run("OIDC flags rejected with DIY backend", func(t *testing.T) {
		// GIVEN
		loginMgr := &oidcLoginManager{creds: creds}
		fs := testFS{MapFS: make(map[string]*fstest.MapFile)}
		testWorkspace := workspace.New(fs, &testPulumiWorkspace{credentials: creds})

		var stdout, stderr bytes.Buffer
		cmd := newLoginCmd(&escCommand{
			command:   "esc",
			login:     loginMgr,
			workspace: testWorkspace,
			environ:   testEnviron{},
			stdout:    &stdout,
			stderr:    &stderr,
		})

		// WHEN - using a DIY backend URL (file://, gs://, s3://, etc.)
		cmd.SetArgs([]string{
			"file:///tmp/state",
			"--oidc-token", "test-oidc-token",
			"--oidc-org", "my-org",
		})
		err := cmd.Execute()

		// THEN
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support Pulumi ESC")
	})

	t.Run("error from LoginWithOIDCToken", func(t *testing.T) {
		// GIVEN
		loginMgr := &oidcLoginManager{
			creds:                   creds,
			loginWithOIDCTokenError: errors.New("OIDC authentication failed"),
		}
		fs := testFS{MapFS: make(map[string]*fstest.MapFile)}
		testWorkspace := workspace.New(fs, &testPulumiWorkspace{credentials: creds})

		var stdout, stderr bytes.Buffer
		cmd := newLoginCmd(&escCommand{
			command:   "esc",
			login:     loginMgr,
			workspace: testWorkspace,
			environ:   testEnviron{},
			stdout:    &stdout,
			stderr:    &stderr,
		})

		// WHEN
		cmd.SetArgs([]string{
			"--oidc-token", "invalid-token",
			"--oidc-org", "my-org",
		})
		err := cmd.Execute()

		// THEN
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "problem logging in")
		assert.Contains(t, err.Error(), "OIDC authentication failed")
	})
}

type errorAuthContextWorkspace struct {
	testPulumiWorkspace
}

func (w *errorAuthContextWorkspace) NewAuthContextForTokenExchange(
	organization, team, user, token, expirationDuration string,
) (pulumi_workspace.AuthContext, error) {
	return pulumi_workspace.AuthContext{}, errors.New("invalid token format")
}

func TestOIDCTokenExchangeAuthContextError(t *testing.T) {
	backend := "https://api.pulumi.com"
	creds := pulumi_workspace.Credentials{
		Current: backend,
		Accounts: map[string]pulumi_workspace.Account{
			backend: {
				Username:    "test-user",
				AccessToken: "access-token",
			},
		},
	}

	t.Run("error from NewAuthContextForTokenExchange", func(t *testing.T) {
		// GIVEN
		loginMgr := &oidcLoginManager{creds: creds}
		fs := testFS{MapFS: make(map[string]*fstest.MapFile)}
		testWorkspace := workspace.New(fs, &errorAuthContextWorkspace{
			testPulumiWorkspace: testPulumiWorkspace{credentials: creds},
		})

		var stdout, stderr bytes.Buffer
		cmd := newLoginCmd(&escCommand{
			command:   "esc",
			login:     loginMgr,
			workspace: testWorkspace,
			environ:   testEnviron{},
			stdout:    &stdout,
			stderr:    &stderr,
		})

		// WHEN
		cmd.SetArgs([]string{
			"--oidc-token", "malformed-token",
			"--oidc-org", "my-org",
		})
		err := cmd.Execute()

		// THEN
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "problem logging in")
		assert.Contains(t, err.Error(), "invalid token format")
	})
}
