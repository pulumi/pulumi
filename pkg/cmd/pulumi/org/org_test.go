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

package org

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func localOrgConfig(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_BACKEND_URL", "https://api.example.com")
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
	original := backend.DefaultLoginManager
	t.Cleanup(func() { backend.DefaultLoginManager = original })
	backend.DefaultLoginManager = &backend.MockLoginManager{
		CurrentF: func(context.Context, pkgWorkspace.Context, diag.Sink, string,
			*workspace.Project, bool,
		) (pkgBackend.Backend, error) {
			return nil, nil
		},
		LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink, string,
			*workspace.Project, bool, bool, colors.Colorization,
		) (pkgBackend.Backend, error) {
			return nil, errors.New("local configuration must not require login")
		},
	}
}

func TestOrgDefaultSources(t *testing.T) {
	for _, args := range [][]string{nil, {"get-default"}} {
		for _, source := range []string{"config", "environment", "backend"} {
			t.Run(source+"/"+stringArgs(args), func(t *testing.T) {
				localOrgConfig(t)
				want := "configured-org"
				if source != "backend" {
					require.NoError(t, workspace.SetBackendConfigDefaultOrg("https://api.example.com", "configured-org"))
				} else {
					want = "backend-org"
					backend.DefaultLoginManager = &backend.MockLoginManager{
						LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink, string,
							*workspace.Project, bool, bool, colors.Colorization,
						) (pkgBackend.Backend, error) {
							return &pkgBackend.MockBackend{
								GetDefaultOrgF: func(context.Context) (string, error) { return "backend-org", nil },
							}, nil
						},
					}
				}
				if source == "environment" {
					t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "environment-org")
					want = "environment-org"
				}
				var out bytes.Buffer
				cmd := NewOrgCmd()
				cmd.SetArgs(args)
				cmd.SetOut(&out)
				require.NoError(t, cmd.Execute())
				assert.Contains(t, out.String(), want+"\n")
			})
		}
	}
}

func stringArgs(args []string) string {
	if len(args) == 0 {
		return "org"
	}
	return args[0]
}

//nolint:paralleltest // localOrgConfig changes the process environment and globals.
func TestOrgSetDefaultWithoutLogin(t *testing.T) {
	localOrgConfig(t)
	cmd := NewOrgCmd()
	cmd.SetArgs([]string{"set-default", "configured-org"})
	require.NoError(t, cmd.Execute())
	config, err := workspace.GetPulumiConfig()
	require.NoError(t, err)
	assert.Equal(t, "configured-org", config.BackendConfig["https://api.example.com"].DefaultOrg)
}

func TestOrgDefaultRejectsDIYBackend(t *testing.T) {
	for _, args := range [][]string{{"get-default"}, {"set-default", "configured-org"}} {
		t.Run(args[0], func(t *testing.T) {
			localOrgConfig(t)
			t.Setenv("PULUMI_BACKEND_URL", "file://"+t.TempDir())
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "environment-org")
			cmd := NewOrgCmd()
			cmd.SetArgs(args)
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "organization")
			assert.NotContains(t, err.Error(), "must not require login")
		})
	}
}

func TestOrgDefaultUsesAgentBackendFallback(t *testing.T) {
	for _, args := range [][]string{nil, {"get-default"}, {"set-default", "configured-org"}} {
		t.Run(stringArgs(args), func(t *testing.T) {
			localOrgConfig(t)
			t.Setenv("PULUMI_BACKEND_URL", "")
			t.Setenv("PULUMI_CREDENTIALS_PATH", "")
			t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
			t.Setenv("AI_AGENT", "test-agent")
			t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
			require.NoError(t, os.Mkdir(filepath.Join(os.Getenv("PULUMI_HOME"), "credentials.json"), 0o700))
			const cloudURL = "https://agent-backend.example.com"
			require.NoError(t, workspace.StoreAgentCredentials(workspace.Credentials{
				Current:      cloudURL,
				AccessTokens: map[string]string{cloudURL: "test-token"},
			}))
			if len(args) == 0 || args[0] != "set-default" {
				require.NoError(t, workspace.SetBackendConfigDefaultOrg(cloudURL, "configured-org"))
			}
			var out bytes.Buffer
			cmd := NewOrgCmd()
			cmd.SetArgs(args)
			cmd.SetOut(&out)
			require.NoError(t, cmd.Execute())
			if len(args) > 0 && args[0] == "set-default" {
				config, err := workspace.GetPulumiConfig()
				require.NoError(t, err)
				assert.Equal(t, "configured-org", config.BackendConfig[cloudURL].DefaultOrg)
			} else {
				assert.Contains(t, out.String(), "configured-org\n")
			}
		})
	}
}
