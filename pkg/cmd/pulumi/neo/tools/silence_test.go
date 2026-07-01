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

//go:build !windows

package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.org/x/sys/unix"
)

// TestSilenceStdIsolatesChildStderr is a regression test for
// https://github.com/pulumi/pulumi-service/issues/44157: while the Neo TUI is
// rendering, stderr from child processes spawned by the in-process engine must not
// reach the terminal. The previous silenceStd only swapped the os.Stdout/os.Stderr
// Go variables, which child processes do not observe — they inherit fd 2 directly.
//
// The test stands a temp file in for the terminal by pointing the real fd 2 at it,
// silences, then spawns a child that writes a marker to its inherited fd 2. With
// only the variable swap the marker leaks into the "terminal"; with fd-level
// redirection it is captured instead.
//
// It does not call t.Parallel: fd 2 is process-global, and Go runs non-parallel
// tests sequentially with parallel ones paused, so the redirect can't race them.
//
//nolint:paralleltest // Redirects the process-global fd 2; must not run in parallel.
func TestSilenceStdIsolatesChildStderr(t *testing.T) {
	const marker = "NEO-CHILD-STDERR-LEAK-MARKER"

	// A stand-in for the terminal: the file the real fd 2 points at for the test.
	termPath := filepath.Join(t.TempDir(), "terminal.txt")
	termFile, err := os.Create(termPath)
	require.NoError(t, err)
	defer termFile.Close()

	// Point the process's real fd 2 at the temp file, restoring it afterwards.
	savedReal, err := redirectFD2(termFile)
	require.NoError(t, err)
	defer func() { require.NoError(t, restoreFD2(savedReal)) }()

	silencer := silenceStd()

	// Spawn a child that writes the marker to its inherited fd 2. os.NewFile wraps
	// the process's current fd 2 (now the silencer's capture pipe), and os/exec
	// dups it onto the child's stderr — exactly how an engine-spawned subprocess
	// inherits the terminal's stderr.
	childStderr := os.NewFile(uintptr(unix.Stderr), "stderr")
	cmd := exec.Command("sh", "-c", "printf %s "+marker+" >&2")
	cmd.Stderr = childStderr
	runErr := cmd.Run()
	runtime.KeepAlive(childStderr)

	captured := silencer.Restore()
	require.NoError(t, runErr)

	termContents, err := os.ReadFile(termPath)
	require.NoError(t, err)

	assert.NotContains(t, string(termContents), marker,
		"child-process stderr leaked onto the terminal (fd 2 was not isolated)")
	assert.Contains(t, captured, marker,
		"child-process stderr should have been captured by the silencer")
}
