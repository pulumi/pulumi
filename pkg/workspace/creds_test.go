// Copyright 2024, Pulumi Corporation.
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

package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCurrentCloudURL(t *testing.T) {
	t.Parallel()

	credsF := func() (workspace.Credentials, error) {
		return workspace.Credentials{Current: "https://credentials.com"}, nil
	}

	tests := []struct {
		name           string
		ws             Context
		e              env.Env
		project        *workspace.Project
		expectedString string
		expectedSource CloudURLSource
		expectedError  error
	}{
		{
			name:           "no project, env, or credentials",
			ws:             &MockContext{},
			e:              env.NewEnv(env.MapStore{}),
			expectedString: "",
			expectedSource: CloudURLSourceNone,
		},
		{
			name: "stored credentials",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e:              env.NewEnv(env.MapStore{}),
			expectedString: "https://credentials.com",
			expectedSource: CloudURLSourceCredentials,
		},
		{
			name: "project setting takes precedence",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e:              env.NewEnv(env.MapStore{}),
			project:        &workspace.Project{Backend: &workspace.ProjectBackend{URL: "https://project.com"}},
			expectedString: "https://project.com",
			expectedSource: CloudURLSourceProject,
		},
		{
			name: "empty project setting falls back to stored credentials",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e:              env.NewEnv(env.MapStore{}),
			project:        &workspace.Project{Backend: &workspace.ProjectBackend{URL: ""}},
			expectedString: "https://credentials.com",
			expectedSource: CloudURLSourceCredentials,
		},
		{
			name: "envvar takes precedence",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e: env.NewEnv(env.MapStore{
				env.BackendURL.Var().Name(): "https://env.com",
			}),
			project:        &workspace.Project{Backend: &workspace.ProjectBackend{URL: "https://project.com"}},
			expectedString: "https://env.com",
			expectedSource: CloudURLSourceEnv,
		},
		{
			name: "report error from stored credentials",
			ws: &MockContext{
				GetStoredCredentialsF: func() (workspace.Credentials, error) {
					return workspace.Credentials{}, assert.AnError
				},
			},
			e:             env.NewEnv(env.MapStore{}),
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			str, source, err := GetCurrentCloudURLWithSource(tt.ws, tt.e, tt.project)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedString, str)
			assert.Equal(t, tt.expectedSource, source)

			// GetCurrentCloudURL must stay in lockstep with the variant above.
			str, err = GetCurrentCloudURL(tt.ws, tt.e, tt.project)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedString, str)
		})
	}
}

func TestGetCurrentCloudURLFallsBackToAgentCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv("CODEX_SANDBOX", "1")

	err := workspace.StoreAgentAccount("https://api.agent.example", workspace.Account{AccessToken: "token-value"}, true)
	require.NoError(t, err)

	ws := &MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	url, err := GetCurrentCloudURLWithAgentFallback(ws, env.Global(), nil)
	require.NoError(t, err)
	assert.Equal(t, "https://api.agent.example", url)
}

func TestGetCurrentCloudURLReturnsEmptyAgentCurrent(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv("CODEX_SANDBOX", "1")

	ws := &MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	url, err := GetCurrentCloudURLWithAgentFallback(ws, env.Global(), nil)
	require.NoError(t, err)
	assert.Empty(t, url)
}

func TestGetCurrentCloudURLReturnsAgentCredentialReadError(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv("CODEX_SANDBOX", "1")
	agentDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(agentDir, []byte("not a directory"), 0o600))
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", agentDir)

	ws := &MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	_, err := GetCurrentCloudURLWithAgentFallback(ws, env.Global(), nil)
	require.ErrorContains(t, err, "could not get cloud url from agent credentials")
}

func TestGetCurrentCloudURLDoesNotFallbackWithExplicitPath(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, "/explicit/pulumi")
	t.Setenv("CODEX_SANDBOX", "1")

	ws := &MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	_, err := GetCurrentCloudURLWithAgentFallback(ws, env.Global(), nil)
	require.ErrorIs(t, err, assert.AnError)
}

func TestGetCurrentCloudURLReturnsDefaultCredentialErrorsOutsideAgents(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")

	ws := &MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	_, err := GetCurrentCloudURLWithAgentFallback(ws, env.Global(), nil)
	require.ErrorIs(t, err, assert.AnError)
}

func TestGetCurrentCloudURLReturnsDefaultCloudURL(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv(env.BackendURL.Var().Name(), "https://api.default-current.example.com")

	url, err := GetCurrentCloudURLWithAgentFallback(&MockContext{}, env.Global(), nil)
	require.NoError(t, err)
	assert.Equal(t, "https://api.default-current.example.com", url)
}
