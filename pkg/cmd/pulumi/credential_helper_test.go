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

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/authtest"
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestMain(m *testing.M) {
	if code, ran := authtest.RunHelper(); ran {
		os.Exit(code)
	}
	if len(os.Args) > 1 && os.Args[1] == "--pulumi-command-test" {
		root, cleanup := NewPulumiCmd()
		root.SetArgs(os.Args[2:])
		err := root.Execute()
		cleanup()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// configureTestCredentialHelper makes the commands a test runs use a fake credential helper that
// answers from responses. It returns the requests the helper has received.
func configureTestCredentialHelper(t *testing.T, responses map[string]any) func() []credentialhelper.Request {
	t.Helper()
	helper := authtest.NewHelper(t, responses)
	args, err := json.Marshal(helper.Args)
	require.NoError(t, err)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", helper.Path)
	t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", string(args))
	return func() []credentialhelper.Request { return helper.Requests(t) }
}

func runCredentialCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	return runCredentialCommandAt(t, executable, args...)
}

func runCredentialCommandAt(t *testing.T, executable string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")
	command := exec.CommandContext(t.Context(), executable, append([]string{
		"--pulumi-command-test", "--non-interactive",
	}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, output)
	}
	return string(output), nil
}

func installCredentialTestExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, name)
	if err := os.Link(executable, path); err != nil {
		require.NoError(t, fsutil.CopyFile(path, executable, nil))
	}
	path, err = filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return path
}

func TestCredentialHelperDiscovery(t *testing.T) {
	for _, test := range []struct {
		name      string
		besideCLI bool
		disabled  bool
	}{
		{name: "PATH"},
		{name: "beside CLI precedes PATH", besideCLI: true},
		{name: "none disables saved and discovered helpers", besideCLI: true, disabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
			t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", "ignored invalid JSON")
			cliDir, pathDir := filepath.Join(t.TempDir(), "cli"), filepath.Join(t.TempDir(), "path")
			cli := installCredentialTestExecutable(t, cliDir, "pulumi")
			pathHelper := installCredentialTestExecutable(t, pathDir, "pulumi-credential-helper")
			t.Setenv("PATH", pathDir)
			responses := map[string]any{
				"initial": map[string]any{"version": 1, "env": map[string]string{"HELPER_TEST_VALUE": "ephemeral"}},
			}
			pathCalls := authtest.NewHelperIn(t, pathDir, responses).Requests
			besideCalls := authtest.NewHelperIn(t, cliDir, responses).Requests
			selected := pathHelper
			if test.besideCLI {
				selected = installCredentialTestExecutable(t, cliDir, "pulumi-credential-helper")
			}
			if test.disabled {
				t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
				require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
					CredentialHelper: &workspace.CredentialHelper{Path: filepath.Join(t.TempDir(), "missing-helper")},
				}))
			}

			_, err := runCredentialCommandAt(t, cli, "version")
			require.NoError(t, err)
			assert.Empty(t, pathCalls(t))
			assert.Empty(t, besideCalls(t))

			backendURL := "file://" + filepath.ToSlash(t.TempDir())
			output, err := runCredentialCommandAt(t, cli, "login", backendURL)
			require.NoError(t, err)
			if test.disabled {
				assert.NotContains(t, output, "Credential helper:")
				assert.Empty(t, pathCalls(t))
				assert.Empty(t, besideCalls(t))
			} else {
				assert.Contains(t, output, "Credential helper: "+selected)
				expected := []credentialhelper.Request{{
					Version: 1, Reason: credentialhelper.Initial,
					SelectedBackendURL: backendURL,
				}}
				if test.besideCLI {
					assert.Equal(t, expected, besideCalls(t))
					assert.Empty(t, pathCalls(t))
				} else {
					assert.Equal(t, expected, pathCalls(t))
					assert.Empty(t, besideCalls(t))
				}
			}
		})
	}
}

func TestCredentialHelperRedactsRefreshedCredentials(t *testing.T) {
	for _, reason := range []credentialhelper.Reason{credentialhelper.Expired, credentialhelper.Rejected} {
		t.Run(string(reason), func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "helper-user")
			const (
				initialToken     = "initial-helper-token-secret"
				initialHeader    = "initial-helper-header-secret"
				refreshedToken   = "refreshed-helper-token-secret"
				refreshedHeader  = "refreshed-helper-header-secret"
				environmentValue = "helper-environment-secret"
			)
			secrets := []string{initialToken, initialHeader, refreshedToken, refreshedHeader, environmentValue}
			// Echo secrets in an ignored field to exercise verbose response-body logging.
			user, err := json.Marshal(map[string]any{
				"githubLogin": "helper-user", "organizations": []string{}, "echoedSecrets": secrets,
			})
			require.NoError(t, err)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "token "+initialToken {
					assert.Equal(t, credentialhelper.Rejected, reason)
					assert.Equal(t, initialHeader, r.Header.Get("X-Helper"))
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				assert.Equal(t, "token "+refreshedToken, r.Header.Get("Authorization"))
				assert.Equal(t, refreshedHeader, r.Header.Get("X-Helper"))
				authtest.ServeBackend(t, w, r, "helper-user", map[string]string{"/api/user": string(user)})
			}))
			defer server.Close()
			initial := map[string]any{
				"version": 1, "backendUrl": server.URL, "accessToken": initialToken,
				"headers": map[string][]string{"X-Helper": {initialHeader}},
				"env":     map[string]string{"HELPER_TEST_VALUE": environmentValue},
			}
			if reason == credentialhelper.Expired {
				initial["expiresAt"] = "2000-01-01T00:00:00Z"
			}
			calls := configureTestCredentialHelper(t, map[string]any{
				"initial": initial,
				string(reason): map[string]any{
					"version": 1, "accessToken": refreshedToken,
					"headers": map[string][]string{"X-Helper": {refreshedHeader}},
				},
			})
			output, err := runCredentialCommand(t, "login", "--logtostderr", "--verbose=11")
			require.NoError(t, err)
			assert.Equal(t, []credentialhelper.Request{
				{Version: 1, Reason: credentialhelper.Initial},
				{Version: 1, Reason: reason, SelectedBackendURL: server.URL},
			}, calls())
			assert.Contains(t, output, "Pulumi API call response body")
			assert.Contains(t, output, "echoedSecrets")
			assert.Contains(t, output, "[credential]")

			logs, err := filepath.Glob(filepath.Join(dirs.Home, "logs", "*.log"))
			require.NoError(t, err)
			require.Len(t, logs, 1)
			compressed, err := os.ReadFile(logs[0])
			require.NoError(t, err)
			reader, err := gzip.NewReader(bytes.NewReader(compressed))
			require.NoError(t, err)
			log, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			for _, contents := range []string{output, string(log)} {
				assert.Contains(t, contents, "Invoking credential helper for initial backend preparation")
				assert.Contains(t, contents, "Refreshing credential helper HTTP credentials")
				for _, secret := range secrets {
					assert.NotContains(t, contents, secret)
				}
			}
			stored, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, workspace.Credentials{Version: 1, Current: server.URL}, stored)
		})
	}
}

func TestCredentialHelperLoginFlags(t *testing.T) {
	for _, decline := range []bool{false, true} {
		t.Run(fmt.Sprintf("decline=%t", decline), func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			backendURL := "file://" + filepath.ToSlash(t.TempDir())
			response := map[string]any{"version": 1}
			if !decline {
				response["env"] = map[string]string{"HELPER_TEST_VALUE": "ephemeral"}
			}
			calls := configureTestCredentialHelper(t, map[string]any{"initial": response})
			path := os.Getenv("PULUMI_CREDENTIAL_HELPER")
			var helperArgs []string
			require.NoError(t, json.Unmarshal([]byte(os.Getenv("PULUMI_CREDENTIAL_HELPER_ARGS")), &helperArgs))
			args := slices.Concat(helperArgs, []string{"literal $(argument), with spaces", ""})
			flags := make([]string, 0, 4+len(args))
			flags = append(flags, "login", backendURL, "--credential-helper", path)
			for _, arg := range args {
				flags = append(flags, "--credential-helper-arg="+arg)
			}
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
			t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", "ignored invalid JSON")
			previous := &workspace.CredentialHelper{Path: filepath.Join(t.TempDir(), "missing-helper")}
			require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
				CredentialHelper: previous,
				Accounts:         map[string]workspace.Account{"https://old.example.com": {AccessToken: "old-token"}},
			}))

			// A failed backend open must leave the existing helper configuration intact.
			blocked := filepath.Join(t.TempDir(), "not-a-directory")
			require.NoError(t, os.WriteFile(blocked, nil, 0o600))
			flags[1] = "file://" + filepath.ToSlash(filepath.Join(blocked, "backend"))
			_, err := runCredentialCommand(t, flags...)
			require.Error(t, err)
			stored, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, previous, stored.CredentialHelper)

			flags[1] = backendURL
			output, err := runCredentialCommand(t, flags...)
			require.NoError(t, err)
			if decline {
				assert.NotContains(t, output, "Credential helper:")
			} else {
				assert.Contains(t, output, "Credential helper: "+path)
			}
			stored, err = workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, &workspace.CredentialHelper{Path: path, Args: args}, stored.CredentialHelper)
			assert.Equal(t, "old-token", stored.Accounts["https://old.example.com"].AccessToken)
			assert.Equal(t, backendURL, stored.Current)

			// The next process uses the saved configuration without explicit flags.
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
			_, err = runCredentialCommand(t, "whoami")
			require.NoError(t, err)
			require.Len(t, calls(), 3)
		})
	}
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCredentialHelperLogout(t *testing.T) {
	for _, args := range [][]string{nil, {"--all"}, {"https://old.example.com"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			calls := configureTestCredentialHelper(t, nil)
			helper := &workspace.CredentialHelper{Path: os.Getenv("PULUMI_CREDENTIAL_HELPER")}
			require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
				Current: "https://old.example.com", CredentialHelper: helper,
				Accounts: map[string]workspace.Account{
					"https://old.example.com":   {AccessToken: "old-token"},
					"https://other.example.com": {AccessToken: "other-token"},
				},
			}))
			_, err := runCredentialCommand(t, append([]string{"logout"}, args...)...)
			require.NoError(t, err)
			stored, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			if len(args) != 0 && args[0] != "--all" {
				assert.Equal(t, helper, stored.CredentialHelper)
			} else {
				assert.Nil(t, stored.CredentialHelper)
			}
			if len(args) == 0 || args[0] != "--all" {
				assert.Equal(t, "other-token", stored.Accounts["https://other.example.com"].AccessToken)
			}
			assert.Empty(t, stored.Current)
			assert.NotContains(t, stored.Accounts, "https://old.example.com")
			assert.Empty(t, calls())
		})
	}
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCredentialHelperRejectsOIDC(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Chdir(t.TempDir())
	calls := configureTestCredentialHelper(t, nil)
	_, err := runCredentialCommand(t, "login", "https://api.example.com", "--oidc-token", "unused-token")
	require.ErrorContains(t, err, "--oidc-token cannot be used with a credential helper")
	assert.Contains(t, err.Error(), "PULUMI_CREDENTIAL_HELPER=none")
	_, err = runCredentialCommand(t, "login", "--oidc-token", "unused-token",
		"--credential-helper", os.Getenv("PULUMI_CREDENTIAL_HELPER"))
	require.ErrorContains(t, err, "credential-helper oidc-token")
	_, err = runCredentialCommand(t, "login", "--credential-helper-arg", "orphaned-argument")
	require.ErrorContains(t, err, "--credential-helper-arg requires --credential-helper")
	_, err = runCredentialCommand(t, "login", "--credential-helper=")
	require.ErrorContains(t, err, "credential helper path must not be empty")
	assert.Empty(t, calls())
}

func TestCredentialHelperKeepsInsecureConnection(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "insecure-user")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token insecure-helper-token", r.Header.Get("Authorization"))
		authtest.ServeBackend(t, w, r, "insecure-user", nil)
	}))
	defer server.Close()
	configureTestCredentialHelper(t, map[string]any{
		"initial": map[string]any{"version": 1, "accessToken": "insecure-helper-token"},
	})
	_, err := runCredentialCommand(t, "login", server.URL, "--insecure")
	require.NoError(t, err)
	// Only the connection setting is stored; the next command needs it to reach the backend.
	stored, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, map[string]workspace.Account{server.URL: {Insecure: true}}, stored.Accounts)
	output, err := runCredentialCommand(t, "whoami")
	require.NoError(t, err)
	assert.Contains(t, output, "insecure-user")
}

func TestCredentialHelperHeadersWithStoredOAuth(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "oauth-user")
	var exchanges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			exchanges.Add(1)
			assert.Equal(t, "rotated-header", r.Header.Get("X-Helper"))
			assert.Empty(t, r.Header.Get("Authorization"))
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "old-refresh-token", r.Form.Get("refresh_token"))
			fmt.Fprint(w, `{"access_token":"rotated-token","refresh_token":"rotated-refresh-token","expires_in":3600}`)
			return
		}
		if r.Header.Get("X-Helper") != "rotated-header" || r.Header.Get("Authorization") != "token rotated-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		authtest.ServeBackend(t, w, r, "oauth-user", map[string]string{
			"/api/esc/environments": `{"environments":[]}`,
		})
	}))
	defer server.Close()
	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken: "old-token", RefreshToken: "old-refresh-token",
	}, true))
	calls := configureTestCredentialHelper(t, map[string]any{
		"initial":  map[string]any{"version": 1, "headers": map[string][]string{"X-Helper": {"initial-header"}}},
		"rejected": map[string]any{"version": 1, "headers": map[string][]string{"X-Helper": {"rotated-header"}}},
	})
	_, err := runCredentialCommand(t, "env", "ls")
	require.NoError(t, err)
	assert.Equal(t, int32(1), exchanges.Load())
	assert.Equal(t, []credentialhelper.Request{
		{Version: 1, Reason: credentialhelper.Initial, SelectedBackendURL: server.URL},
		{Version: 1, Reason: credentialhelper.Rejected, SelectedBackendURL: server.URL},
	}, calls())
	stored, err := workspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "rotated-token", stored.AccessToken)
	assert.Equal(t, "rotated-refresh-token", stored.RefreshToken)
}
