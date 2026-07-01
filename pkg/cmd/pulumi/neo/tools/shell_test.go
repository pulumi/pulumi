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

package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShell_ExecuteCapturesStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	res, err := sh.Invoke(t.Context(), "shell_execute", json.RawMessage(`{"command":"echo hi"}`))
	require.NoError(t, err)
	r := res.(ShellResult)
	assert.Equal(t, "hi\n", r.Stdout)
	assert.Equal(t, 0, r.ExitCode)
}

func TestShell_ExecuteHonorsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	sh.DefaultTimeout = 50 * time.Millisecond
	res, err := sh.Invoke(t.Context(), "shell_execute", json.RawMessage(`{"command":"sleep 5"}`))
	require.Error(t, err, "timeout must surface as a non-nil error so the TUI marks the call failed")
	assert.Contains(t, err.Error(), "timed out")
	require.NotNil(t, res, "partial result must still be returned alongside the timeout error")
	assert.True(t, res.(ShellResult).TimedOut)
}

func TestShell_ExecuteHonorsAgentTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	start := time.Now()
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"sleep 5","timeout":0.1}`))
	require.Error(t, err)
	require.NotNil(t, res)
	assert.True(t, res.(ShellResult).TimedOut)
	assert.Less(t, time.Since(start), 5*time.Second, "agent-supplied timeout was ignored")
}

func TestShell_ExecuteAgentTimeoutOverridesDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	sh.DefaultTimeout = time.Hour // way longer than the agent value below
	start := time.Now()
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"sleep 5","timeout":0.1}`))
	require.Error(t, err)
	require.NotNil(t, res)
	assert.True(t, res.(ShellResult).TimedOut)
	assert.Less(t, time.Since(start), 5*time.Second, "agent timeout did not override DefaultTimeout")
}

func TestShell_ExecuteUnblocksOnOrphanedChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	// `sleep 30 &; wait` keeps a grandchild holding the inherited stdout/stderr
	// pipes open well past the deadline. Without process-group kill +
	// WaitDelay, cmd.Run() blocks for the full 30s instead of returning when
	// the timeout fires.
	start := time.Now()
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"sleep 30 & wait","timeout":0.2}`))
	require.Error(t, err)
	require.NotNil(t, res)
	assert.True(t, res.(ShellResult).TimedOut)
	// Allow generous slack for the WaitDelay grace period (5s) plus CI noise.
	assert.Less(t, time.Since(start), 15*time.Second, "shell hung on orphaned child")
}

func TestShell_ExecuteRejectsNegativeTimeout(t *testing.T) {
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	_, err = sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"echo hi","timeout":-1}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
}

func TestShell_ExecuteCapturesNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	res, err := sh.Invoke(t.Context(), "shell_execute", json.RawMessage(`{"command":"exit 3"}`))
	require.NoError(t, err)
	assert.Equal(t, 3, res.(ShellResult).ExitCode)
}

func TestShell_ExecuteHonorsCwdSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(sub, 0o755))

	sh, err := NewShell(root)
	require.NoError(t, err)
	// Resolve sub through EvalSymlinks so the comparison matches on macOS where
	// /var -> /private/var.
	resolvedSub, err := filepath.EvalSymlinks(sub)
	require.NoError(t, err)
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"pwd","cwd":"`+sub+`"}`))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(res.(ShellResult).Stdout, resolvedSub))
}

func TestShell_ExecuteRejectsCwdOutsideRoot(t *testing.T) {
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	outside := t.TempDir()
	_, err = sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(fmt.Sprintf(`{"command":"echo hi","cwd":%q}`, outside)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the allowed roots")
}

func TestShell_ExecuteRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	_, err = sh.Invoke(t.Context(), "shell_execute", json.RawMessage(`{"command":""}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty command")
}

func TestShell_RejectsCwdSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	require.NoError(t, os.Symlink(outside, link))

	sh, err := NewShell(root)
	require.NoError(t, err)
	_, err = sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(fmt.Sprintf(`{"command":"echo hi","cwd":%q}`, link)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the allowed roots")
}

func TestShell_TruncatesLargeOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	// Generate more than maxOutputBytes of stdout.
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"dd if=/dev/zero bs=1048576 count=2 2>/dev/null | tr '\\0' 'A'"}`))
	require.NoError(t, err)
	r := res.(ShellResult)
	assert.True(t, r.Truncated)
	assert.LessOrEqual(t, len(r.Stdout), maxOutputBytes)
}

func TestShell_RejectsUnknownMethod(t *testing.T) {
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	_, err = sh.Invoke(t.Context(), "run", json.RawMessage(`{"command":"echo"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown shell method")
}

func TestShell_AcceptsCwdUnderExtraRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	t.Parallel()

	// Primary cwd and scratch are unrelated directories. Passing scratch as an
	// extra root mirrors what neo.go does for /tmp: commands launched there must
	// run successfully even though scratch is outside the primary cwd.
	cwd := t.TempDir()
	scratch := t.TempDir()
	sh, err := NewShell(cwd, scratch)
	require.NoError(t, err)

	resolvedScratch, err := filepath.EvalSymlinks(scratch)
	require.NoError(t, err)
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(fmt.Sprintf(`{"command":"pwd","cwd":%q}`, scratch)))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(res.(ShellResult).Stdout, resolvedScratch))
}

func TestShell_RejectsCwdOutsideRootAndExtras(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	scratch := t.TempDir()
	sh, err := NewShell(cwd, scratch)
	require.NoError(t, err)

	// A third unrelated directory must still be rejected even with extras configured.
	other := t.TempDir()
	_, err = sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(fmt.Sprintf(`{"command":"echo hi","cwd":%q}`, other)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the allowed roots")
}

func TestNewShell_RejectsMissingExtraRoot(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "nope")
	_, err := NewShell(t.TempDir(), missing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra root")
}

func TestNewShell_RejectsExtraRootThatIsAFile(t *testing.T) {
	t.Parallel()

	// An extra root that points at a file (not a directory) must be rejected.
	notADir := filepath.Join(t.TempDir(), "regular-file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o600))

	_, err := NewShell(t.TempDir(), notADir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory")
}

func TestShell_InjectsAIAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh semantics")
	}
	// Set AI_AGENT in the parent so we can prove the shell handler overrides
	// (not just appends to) a pre-existing value. t.Setenv precludes t.Parallel.
	t.Setenv("AI_AGENT", "claude")

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"printenv AI_AGENT"}`))
	require.NoError(t, err)
	assert.Equal(t, "neo\n", res.(ShellResult).Stdout)
}

func TestChildEnvWithAgent(t *testing.T) {
	t.Parallel()

	t.Run("empty parent", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"AI_AGENT=neo"}, childEnvWithAgent(nil))
	})

	t.Run("no AI_AGENT in parent", func(t *testing.T) {
		t.Parallel()
		parent := []string{"PATH=/usr/bin", "HOME=/home/x"}
		got := childEnvWithAgent(parent)
		assert.Equal(t, []string{"PATH=/usr/bin", "HOME=/home/x", "AI_AGENT=neo"}, got)
	})

	t.Run("strips existing AI_AGENT", func(t *testing.T) {
		t.Parallel()
		parent := []string{"PATH=/usr/bin", "AI_AGENT=claude", "HOME=/home/x"}
		got := childEnvWithAgent(parent)
		assert.Equal(t, []string{"PATH=/usr/bin", "HOME=/home/x", "AI_AGENT=neo"}, got)
		// No duplicate AI_AGENT entries.
		count := 0
		for _, kv := range got {
			if strings.HasPrefix(kv, "AI_AGENT=") {
				count++
			}
		}
		assert.Equal(t, 1, count)
	})
}
