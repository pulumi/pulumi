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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestMain(m *testing.M) {
	if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == "pulumi-credential-helper" ||
		(len(os.Args) > 1 && os.Args[1] == "--credential-helper-test") {
		var request credentialhelper.Request
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, os.Getenv("PULUMI_TEST_HELPER_STDERR"))
		fmt.Fprintln(os.Stdout, os.Getenv("PULUMI_TEST_HELPER_"+strings.ToUpper(string(request.Reason))))
		os.Exit(0)
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

func runCredentialCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")
	command := exec.CommandContext(t.Context(), executable, append([]string{
		"--pulumi-command-test", "--non-interactive",
	}, args...)...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestCredentialHelperLifecycle(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_HELPER", filepath.Join(t.TempDir(), "missing-helper"))
	t.Setenv("PULUMI_TEST_HELPER_INITIAL", `{"version":1,"env":{"HELPER_TEST_VALUE":"ephemeral"}}`)
	t.Setenv("PULUMI_TEST_HELPER_STDERR", "helper diagnostic\n")
	executable, err := os.Executable()
	require.NoError(t, err)
	name := "pulumi-credential-helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, name)
	if err := os.Link(executable, helper); err != nil {
		require.NoError(t, fsutil.CopyFile(helper, executable, nil))
	}
	helper, err = filepath.EvalSymlinks(helper)
	require.NoError(t, err)
	t.Setenv("PATH", dir)

	output, err := runCredentialCommand(t, "version")
	require.NoError(t, err, output)
	assert.NotContains(t, output, "helper diagnostic")
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
	backendURL := "file://" + filepath.ToSlash(t.TempDir())
	output, err = runCredentialCommand(t, "login", backendURL)
	require.NoError(t, err, output)
	assert.Equal(t, 1, strings.Count(output, "helper diagnostic"))

	// Explicit login saves the helper only after the backend opens, even when the helper declines.
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
	t.Setenv("PULUMI_TEST_HELPER_INITIAL", `{}`)
	stored, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	previous := &workspace.CredentialHelper{Path: filepath.Join(t.TempDir(), "missing-helper")}
	stored.CredentialHelper = previous
	require.NoError(t, workspace.StoreCredentials(stored))
	literal := "literal $(argument), with spaces"
	flags := []string{"login", backendURL, "--credential-helper", helper, "--credential-helper-arg=" + literal}
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	flags[1] = "file://" + filepath.ToSlash(filepath.Join(blocked, "backend"))
	_, err = runCredentialCommand(t, flags...)
	require.Error(t, err)
	stored, err = workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, previous, stored.CredentialHelper)
	flags[1] = backendURL
	output, err = runCredentialCommand(t, flags...)
	require.NoError(t, err, output)
	stored, err = workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, &workspace.CredentialHelper{Path: helper, Args: []string{literal}}, stored.CredentialHelper)

	// A fresh command discovers the saved helper without PATH or explicit configuration.
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
	t.Setenv("PATH", t.TempDir())
	output, err = runCredentialCommand(t, "whoami")
	require.NoError(t, err, output)
	assert.Equal(t, 1, strings.Count(output, "helper diagnostic"))

	// Diagnostics report the saved helper configuration.
	t.Setenv("PULUMI_TEST_HELPER_STDERR", "")
	output, err = runCredentialCommand(t, "whoami", "--json")
	require.NoError(t, err, output)
	var identity map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &identity))
	assert.Equal(t, helper, identity["credentialHelper"])
	output, err = runCredentialCommand(t, "about", "--json")
	require.NoError(t, err, output)
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &summary))
	assert.Equal(t, map[string]any{"path": helper, "source": "saved"}, summary["credentialHelper"])
}

func TestCredentialHelperRedactsRefreshedCredentials(t *testing.T) {
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
	user, err := json.Marshal(map[string]any{"githubLogin": "helper-user", "echoedSecrets": secrets})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "token "+initialToken {
			assert.Equal(t, initialHeader, r.Header.Get("X-Helper"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.Equal(t, "token "+refreshedToken, r.Header.Get("Authorization"))
		assert.Equal(t, refreshedHeader, r.Header.Get("X-Helper"))
		body := []byte(`{}`)
		if r.URL.Path == "/api/user" {
			body = user
		} else {
			assert.Equal(t, "/api/capabilities", r.URL.Path)
		}
		_, err := w.Write(body)
		require.NoError(t, err)
	}))
	defer server.Close()
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", executable)
	t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", `["--credential-helper-test"]`)
	for reason, response := range map[string]map[string]any{
		"INITIAL": {
			"version": 1, "backendUrl": server.URL, "accessToken": initialToken,
			"headers": http.Header{"X-Helper": {initialHeader}}, "env": map[string]string{"HELPER_TEST_VALUE": environmentValue},
		},
		"REJECTED": {"version": 1, "accessToken": refreshedToken, "headers": http.Header{"X-Helper": {refreshedHeader}}},
	} {
		data, err := json.Marshal(response)
		require.NoError(t, err)
		t.Setenv("PULUMI_TEST_HELPER_"+reason, string(data))
	}
	output, err := runCredentialCommand(t, "login", "--logtostderr", "--verbose=11")
	require.NoError(t, err, output)
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
}
