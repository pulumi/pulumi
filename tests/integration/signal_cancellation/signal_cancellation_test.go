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

package signal_cancellation

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/require"
)

// TestSignalCancellation verifies that when pulumi receives SIGINT during `pulumi up`, the language host delivers
// SIGINT to the running program, giving it a chance to clean up gracefully.
func TestSignalCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sending SIGINT to a process is not supported on Windows")
	}
	t.Parallel()
	for _, lang := range []string{"go", "python", "nodejs"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			sentinelDir := t.TempDir()

			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()

			e.ImportDirectory(lang)
			e.SetEnvVars("SENTINEL_DIR=" + sentinelDir)

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "stack", "init", "test")
			e.RunCommand("pulumi", "install")

			cmd := e.SetupCommandIn(e.Context(), e.CWD, "pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			require.NoError(t, cmd.Start(), "failed to start pulumi up")

			startedPath := filepath.Join(sentinelDir, "started")
			require.Eventually(t, func() bool {
				_, err := os.Stat(startedPath)
				return err == nil
			}, 120*time.Second, 100*time.Millisecond, "program did not write 'started' sentinel")

			require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

			waitDone := make(chan error, 1)
			go func() { waitDone <- cmd.Wait() }()
			select {
			case err := <-waitDone:
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr, "expected an ExitError")
				require.False(t, exitErr.Success(), "expected non-successful exit")
				t.Logf("stderr: %s", exitErr.Stderr)
			case <-time.After(120 * time.Second):
				require.Fail(t, "timed out waiting for pulumi to exit after SIGINT")
			}

			_, statErr := os.Stat(filepath.Join(sentinelDir, "graceful-shutdown"))
			require.NoError(t, statErr)
		})
	}
}

// TestSignalCancellationForceKill verifies that when a program ignores SIGINT, the language host force-kills it.
func TestSignalCancellationForceKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sending SIGINT to a process is not supported on Windows")
	}
	t.Parallel()
	for _, lang := range []string{"go_ignore", "python_ignore", "nodejs_ignore"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			sentinelDir := t.TempDir()

			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()

			e.ImportDirectory(lang)
			e.SetEnvVars("SENTINEL_DIR=" + sentinelDir)

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "stack", "init", "test")
			e.RunCommand("pulumi", "install")

			cmd := e.SetupCommandIn(e.Context(), e.CWD, "pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			require.NoError(t, cmd.Start(), "failed to start pulumi up")

			startedPath := filepath.Join(sentinelDir, "started")
			require.Eventually(t, func() bool {
				_, err := os.Stat(startedPath)
				return err == nil
			}, 120*time.Second, 100*time.Millisecond, "program did not write 'started' sentinel")

			require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

			waitDone := make(chan error, 1)
			go func() { waitDone <- cmd.Wait() }()
			select {
			case err := <-waitDone:
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr, "expected an ExitError")
				require.False(t, exitErr.Success(), "expected non-successful exit")
				t.Logf("stderr: %s", exitErr.Stderr)
			case <-time.After(120 * time.Second):
				require.Fail(t, "timed out waiting for pulumi to exit after SIGINT")
			}
		})
	}
}

// TestPluginSignalCancellation verifies that when pulumi receives SIGINT during `pulumi up`, the engine sends
// a Cancel RPC to plugins, giving them a chance to clean up.
func TestPluginSignalCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sending SIGINT to a process is not supported on Windows")
	}
	t.Parallel()
	for _, lang := range []string{"plugin_go", "plugin_python", "plugin_nodejs"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			sentinelDir := t.TempDir()

			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()

			e.ImportDirectory(lang)
			e.SetEnvVars("SENTINEL_DIR=" + sentinelDir)

			providerDir := filepath.Join(e.CWD, "provider")
			programDir := filepath.Join(e.CWD, "program")

			_, stderr, err := e.GetCommandResultsIn(providerDir, "pulumi", "install")
			require.NoError(t, err, "failed to install provider deps: %s", stderr)

			e.CWD = programDir
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "stack", "init", "test")
			e.RunCommand("pulumi", "install")

			cmd := e.SetupCommandIn(e.Context(), e.CWD, "pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			require.NoError(t, cmd.Start(), "failed to start pulumi up")

			startedPath := filepath.Join(sentinelDir, "started")
			require.Eventually(t, func() bool {
				_, err := os.Stat(startedPath)
				return err == nil
			}, 120*time.Second, 100*time.Millisecond, "plugin did not write 'started' sentinel")

			require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

			waitDone := make(chan error, 1)
			go func() { waitDone <- cmd.Wait() }()
			select {
			case err := <-waitDone:
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				require.False(t, exitErr.Success())
				t.Logf("stderr: %s", exitErr.Stderr)
			case <-time.After(120 * time.Second):
				require.Fail(t, "timed out waiting for pulumi to exit after SIGINT")
			}

			_, statErr := os.Stat(filepath.Join(sentinelDir, "graceful-shutdown"))
			require.NoError(t, statErr)
		})
	}
}

// TestPluginSignalCancellationForceKill verifies that when a plugin ignores the Cancel RPC,
// it is force-killed during Close.
func TestPluginSignalCancellationForceKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sending SIGINT to a process is not supported on Windows")
	}
	t.Parallel()
	for _, lang := range []string{"plugin_go_ignore", "plugin_python_ignore", "plugin_nodejs_ignore"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			sentinelDir := t.TempDir()

			e := ptesting.NewEnvironment(t)
			defer e.DeleteIfNotFailed()

			e.ImportDirectory(lang)
			e.SetEnvVars("SENTINEL_DIR=" + sentinelDir)

			providerDir := filepath.Join(e.CWD, "provider")
			programDir := filepath.Join(e.CWD, "program")

			_, stderr, err := e.GetCommandResultsIn(providerDir, "pulumi", "install")
			require.NoError(t, err, "failed to install provider deps: %s", stderr)

			e.CWD = programDir
			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "stack", "init", "test")
			e.RunCommand("pulumi", "install")

			cmd := e.SetupCommandIn(e.Context(), e.CWD, "pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			require.NoError(t, cmd.Start())

			startedPath := filepath.Join(sentinelDir, "started")
			require.Eventually(t, func() bool {
				_, err := os.Stat(startedPath)
				return err == nil
			}, 120*time.Second, 100*time.Millisecond, "plugin did not write 'started' sentinel")

			require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

			waitDone := make(chan error, 1)
			go func() { waitDone <- cmd.Wait() }()
			select {
			case err := <-waitDone:
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				require.False(t, exitErr.Success())
				t.Logf("stderr: %s", exitErr.Stderr)
			case <-time.After(120 * time.Second):
				require.Fail(t, "timed out waiting for pulumi to exit after SIGINT")
			}
		})
	}
}
