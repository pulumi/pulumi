// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/workspace"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
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
