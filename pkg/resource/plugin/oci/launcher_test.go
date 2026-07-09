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

package oci

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRuntime writes a shell script that records each invocation's argv (one
// line per call) to recordFile and responds per subcommand.
func stubRuntime(t *testing.T) (*Runtime, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := filepath.Join(dir, "docker")
	err := os.WriteFile(script, []byte(`#!/bin/sh
echo "$@" >> "`+record+`"
case "$1" in
  run) echo "cid0123456789" ;;
  port) echo "127.0.0.1:49321" ;;
  inspect) echo "true" ;;
  logs) echo "pack logs here" ;;
  stop) : ;;
esac
`), 0o600)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(script, 0o700))
	return &Runtime{Path: script, Name: "docker"}, record
}

func recorded(t *testing.T, record string) []string {
	t.Helper()
	b, err := os.ReadFile(record)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestLaunchHostMode(t *testing.T) {
	t.Parallel()
	rt, record := stubRuntime(t)
	c, err := rt.Launch(t.Context(), LaunchOptions{
		Image:    "ghcr.io/acme/pack@sha256:abc",
		PackName: "security",
		Mode:     ModeHost,
	})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:49321", c.Addr)

	calls := recorded(t, record)
	require.Len(t, calls, 2) // run, port
	runCall := calls[0]
	assert.Contains(t, runCall, "run --detach --rm --pull=never")
	assert.Contains(t, runCall, "-e PULUMI_POLICY_PORT=20851")
	assert.Contains(t, runCall, "-p 127.0.0.1:0:20851")
	assert.Contains(t, runCall, "--add-host=host.docker.internal:host-gateway")
	assert.Contains(t, runCall, "--label com.pulumi.policy-pack=security")
	assert.True(t, strings.HasSuffix(runCall, "ghcr.io/acme/pack@sha256:abc"))
	assert.Contains(t, calls[1], "port cid0123456789 20851")
}

func TestLaunchSiblingMode(t *testing.T) {
	t.Parallel()
	rt, record := stubRuntime(t)
	c, err := rt.Launch(t.Context(), LaunchOptions{
		Image:           "ghcr.io/acme/pack@sha256:abc",
		PackName:        "security",
		Mode:            ModeSibling,
		SelfContainerID: "selfctr",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(c.Addr, "127.0.0.1:"))

	calls := recorded(t, record)
	require.Len(t, calls, 1) // run only; no port mapping to discover
	assert.Contains(t, calls[0], "--network container:selfctr")
	assert.NotContains(t, calls[0], "-p 127.0.0.1")
	assert.NotContains(t, calls[0], "--add-host")
	// The chosen free port is passed into the container.
	assert.Contains(t, calls[0], "-e PULUMI_POLICY_PORT=")
}

func TestLaunchPassesEnvSorted(t *testing.T) {
	t.Parallel()
	rt, record := stubRuntime(t)
	_, err := rt.Launch(t.Context(), LaunchOptions{
		Image: "img", PackName: "p", Mode: ModeHost,
		Env: map[string]string{"B_VAR": "2", "A_VAR": "1"},
	})
	require.NoError(t, err)
	runCall := recorded(t, record)[0]
	assert.Less(t, strings.Index(runCall, "A_VAR=1"), strings.Index(runCall, "B_VAR=2"))
}

func TestContainerLifecycle(t *testing.T) {
	t.Parallel()
	rt, record := stubRuntime(t)
	c, err := rt.Launch(t.Context(), LaunchOptions{Image: "img", PackName: "p", Mode: ModeHost})
	require.NoError(t, err)
	assert.True(t, c.Running(t.Context()))
	assert.Equal(t, "pack logs here", strings.TrimSpace(c.Logs(t.Context())))
	require.NoError(t, c.Close())
	calls := recorded(t, record)
	assert.Contains(t, calls[len(calls)-1], "stop")
	assert.Contains(t, calls[len(calls)-1], "cid0123456789")
}

func TestLaunchRunFailureIncludesOutput(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
echo "manifest for img not found" >&2
exit 125
`), 0o600))
	require.NoError(t, os.Chmod(script, 0o700))
	rt := &Runtime{Path: script, Name: "docker"}
	_, err := rt.Launch(t.Context(), LaunchOptions{Image: "img", PackName: "p", Mode: ModeHost})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest for img not found")
}

func TestEngineAddressFor(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "host.docker.internal:5005", EngineAddressFor(ModeHost, "127.0.0.1:5005"))
	assert.Equal(t, "127.0.0.1:5005", EngineAddressFor(ModeSibling, "127.0.0.1:5005"))
}
