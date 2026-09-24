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

package config

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestLocalConfigDoesNotLogin(t *testing.T) {
	for _, name := range agentdetect.DetectionEnvVars() {
		t.Setenv(name, "")
	}
	t.Setenv("AI_AGENT", "test-agent")
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("PULUMI_BACKEND_URL", "https://api.pulumi.com")
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
	original := cmdBackend.DefaultLoginManager
	t.Cleanup(func() { cmdBackend.DefaultLoginManager = original })
	cmdBackend.DefaultLoginManager = &cmdBackend.MockLoginManager{
		LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project,
			bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return nil, errors.New("unexpected login for local configuration")
		},
	}
	for _, tt := range []struct {
		name           string
		args           []string
		env            string
		output         string
		contains       string
		absent         string
		plaintextOnly  bool
		login          bool
		stack          string
		current        bool
		customFile     bool
		human          bool
		token          bool
		storedToken    bool
		workspaceStack bool
		missingFile    bool
		wantError      string
	}{
		{name: "get", args: []string{"get", "plain"}, output: "hello"},
		{name: "project-default", args: []string{"get", "fallback"}, output: "default-value"},
		{
			name: "missing-key", args: []string{"get", "missing"},
			wantError: "configuration key 'missing' not found",
		},
		{name: "list", output: "[secret]"},
		{name: "list-json", args: []string{"--json"}, output: `"secret": true`},
		{name: "set", args: []string{"set", "plain", "updated"}, contains: "local:plain: updated"},
		{name: "set-all", args: []string{"set-all", "--plaintext", "plain=updated"}, contains: "local:plain: updated"},
		{name: "remove-secret", args: []string{"rm", "secret"}, absent: "ciphertext"},
		{name: "remove-all", args: []string{"rm-all", "plain", "secret"}, absent: "ciphertext"},
		{name: "copy", args: []string{"cp", "plain", "--dest", "other"}},
		{name: "copy-all", args: []string{"cp", "--dest", "other"}, plaintextOnly: true},
		{name: "get-path", args: []string{"get", "--path", "plain"}, output: "hello"},
		{name: "qualified", args: []string{"get", "plain"}, stack: "org/local/dev", output: "hello"},
		{name: "current", args: []string{"get", "plain"}, current: true, output: "hello"},
		{name: "custom-file", args: []string{"get", "plain"}, customFile: true, output: "hello"},
		{
			name: "set-all-json", args: []string{"set-all", "--json", `{"local:plain":{"value":"updated"}}`},
			contains: "local:plain: updated",
		},
		{name: "get-secret", args: []string{"get", "secret"}, login: true},
		{name: "show-secrets", args: []string{"--show-secrets"}, login: true},
		{name: "set-secret", args: []string{"set", "secret", "value", "--secret"}, login: true},
		{name: "set-all-secret", args: []string{"set-all", "--secret", "secret=value"}, login: true},
		{
			name: "set-all-json-secret", args: []string{"set-all", "--json", `{"local:secret":{"value":"value","secret":true}}`},
			login: true,
		},
		{name: "copy-secret", args: []string{"cp", "secret", "--dest", "other"}, login: true},
		{name: "copy-all-secret", args: []string{"cp", "--dest", "other"}, login: true},
		{name: "get-environment", args: []string{"get", "plain"}, env: "environment:\n  - org/env\n", login: true},
		{name: "list-environment", env: "environment:\n  - org/env\n", login: true},
		{
			name: "env-remove-remaining", args: []string{"env", "rm", "org/env", "--yes"},
			env: "environment:\n  - org/env\n  - org/remaining\n", login: true,
		},
		{
			name: "env-remove-inline-values", args: []string{"env", "rm", "org/env", "--yes"},
			env:   "environment:\n  imports:\n    - org/env\n  values:\n    pulumiConfig:\n      local:plain: env-value\n",
			login: true,
		},
		{
			name: "env-remove-show-secrets", args: []string{"env", "rm", "org/env", "--yes", "--show-secrets"},
			env: "environment:\n  - org/env\n", login: true,
		},
		{name: "refresh", args: []string{"refresh"}, login: true},
		{name: "other-project", args: []string{"get", "plain"}, stack: "org/other/dev", login: true},
		{name: "human", args: []string{"get", "plain"}, human: true, login: true},
		{name: "token", args: []string{"get", "plain"}, token: true, login: true},
		{name: "stored-token", args: []string{"get", "plain"}, storedToken: true, login: true},
		{name: "workspace-stack", args: []string{"get", "plain"}, workspaceStack: true, output: "hello"},
		{name: "missing-file", args: []string{"get", "plain"}, missingFile: true, login: true},
		{name: "env-list", args: []string{"env", "ls"}, env: "environment:\n  - org/env\n", output: "org/env"},
		{
			name: "env-remove", args: []string{"env", "rm", "org/env", "--yes"},
			env: "environment:\n  - org/env\n", absent: "org/env",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if tt.human {
				t.Setenv("AI_AGENT", "")
			}
			if tt.token {
				t.Setenv("PULUMI_ACCESS_TOKEN", "test-token")
			}
			if tt.storedToken {
				t.Setenv("PULUMI_HOME", t.TempDir())
				require.NoError(t, workspace.StoreAccount("https://api.pulumi.com",
					workspace.Account{AccessToken: "test-token", Username: "test-user", LastValidatedAt: time.Now()}, true))
			}
			projectYAML := "name: local\nruntime: nodejs\nconfig:\n  fallback:\n    default: default-value\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(projectYAML), 0o600))
			configPath := filepath.Join(dir, "Pulumi.dev.yaml")
			if tt.customFile {
				configPath = filepath.Join(dir, "custom.yaml")
			}
			stackYAML := tt.env + "config:\n  local:plain: hello\n"
			if !tt.plaintextOnly {
				stackYAML += "  local:secret:\n    secure: ciphertext\n"
			}
			if !tt.missingFile {
				require.NoError(t, os.WriteFile(configPath, []byte(stackYAML), 0o600))
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.other.yaml"), []byte("config: {}\n"), 0o600))
			cmd := NewConfigCmd(pkgWorkspace.Instance)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			stackName := tt.stack
			if stackName == "" {
				stackName = "dev"
			}
			args := append([]string{}, tt.args...)
			if tt.workspaceStack {
				w, err := pkgWorkspace.Instance.New("")
				require.NoError(t, err)
				w.Settings().SetStackForBackend("https://api.pulumi.com", "org/local/dev")
				require.NoError(t, w.Save())
			} else if tt.current {
				t.Setenv("PULUMI_STACK", stackName)
			} else {
				args = append(args, "--stack", stackName)
			}
			if tt.customFile {
				args = append(args, "--config-file", configPath)
			}
			cmd.SetArgs(args)
			if tt.login {
				require.ErrorContains(t, cmd.ExecuteContext(t.Context()), "unexpected login for local configuration")
				if tt.missingFile {
					require.NoFileExists(t, configPath)
					return
				}
				content, err := os.ReadFile(configPath)
				require.NoError(t, err)
				require.Equal(t, stackYAML, string(content))
				return
			}
			if tt.wantError != "" {
				require.ErrorContains(t, cmd.ExecuteContext(t.Context()), tt.wantError)
				return
			}
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			require.NotContains(t, out.String(), "ciphertext")
			if tt.output != "" {
				require.Contains(t, out.String(), tt.output)
			}
			content, err := os.ReadFile(configPath)
			require.NoError(t, err)
			if tt.contains != "" {
				require.Contains(t, string(content), tt.contains)
			}
			if tt.absent != "" {
				require.NotContains(t, string(content), tt.absent)
			}
			if tt.name == "copy" || tt.name == "copy-all" {
				project, _, err := pkgWorkspace.Instance.ReadProject("")
				require.NoError(t, err)
				ps, err := workspace.LoadProjectStack(cmdutil.Diag(), project, filepath.Join(dir, "Pulumi.other.yaml"))
				require.NoError(t, err)
				require.Equal(t, config.NewValue("hello"), ps.Config[config.MustMakeKey("local", "plain")])
			}
		})
	}
}

func TestLocalConfigWithStaleCredentials(t *testing.T) {
	t.Setenv("AI_AGENT", "test-agent")
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	original := cmdBackend.DefaultLoginManager
	t.Cleanup(func() { cmdBackend.DefaultLoginManager = original })
	cmdBackend.DefaultLoginManager = &cmdBackend.MockLoginManager{
		LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project,
			bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return nil, errors.New("unexpected login for local configuration")
		},
	}
	for _, tt := range []struct {
		name        string
		agent       bool
		activeClaim bool
	}{
		{name: "expired-agent", agent: true},
		{name: "invalid-user"},
		{name: "active-claim", agent: true, activeClaim: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_HOME", t.TempDir())
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/user" || tt.agent {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(http.StatusUnauthorized)
			}))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			expired := time.Now().Add(-time.Hour)
			if tt.agent {
				require.NoError(t, workspace.StoreAgentAccount(server.URL, workspace.Account{
					AccessToken: "expired", TokenInformation: &workspace.TokenInformation{ExpiresAt: &expired},
				}, true))
				claimExpiry := expired
				if tt.activeClaim {
					claimExpiry = time.Now().Add(time.Hour)
				}
				require.NoError(t, workspace.StoreAgentClaim(workspace.AgentClaim{
					CloudURL: server.URL, ClaimURL: server.URL + "/claim", ValidUntil: claimExpiry,
				}))
			} else {
				require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{AccessToken: "invalid"}, true))
			}
			dir := t.TempDir()
			t.Chdir(dir)
			require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("name: local\nruntime: nodejs\n"), 0o600))
			require.NoError(t, os.WriteFile("Pulumi.dev.yaml", []byte("config:\n  local:plain: hello\n"), 0o600))
			cmd := NewConfigCmd(pkgWorkspace.Instance)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"get", "plain", "--stack", "dev"})
			err := cmd.ExecuteContext(t.Context())
			if tt.activeClaim {
				require.ErrorContains(t, err, "claim the account to regain access")
				claim, err := workspace.GetAgentClaim()
				require.NoError(t, err)
				require.Equal(t, server.URL+"/claim", claim.ClaimURL)
			} else {
				require.NoError(t, err)
				require.Equal(t, "hello\n", out.String())
			}
		})
	}
}
