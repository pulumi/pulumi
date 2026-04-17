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
	m := res.(map[string]any)
	assert.Equal(t, "hi\n", m["stdout"])
	assert.Equal(t, 0, m["exit_code"])
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
	require.NoError(t, err)
	assert.Equal(t, true, res.(map[string]any)["timed_out"])
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
	assert.Equal(t, 3, res.(map[string]any)["exit_code"])
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
	assert.True(t, strings.HasPrefix(res.(map[string]any)["stdout"].(string), resolvedSub))
}

func TestShell_ExecuteRejectsCwdOutsideRoot(t *testing.T) {
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	outside := t.TempDir()
	_, err = sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(fmt.Sprintf(`{"command":"echo hi","cwd":%q}`, outside)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
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
	assert.Contains(t, err.Error(), "outside the working directory")
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
	m := res.(map[string]any)
	assert.True(t, m["truncated"].(bool))
	assert.LessOrEqual(t, len(m["stdout"].(string)), maxOutputBytes)
}

func TestShell_RejectsUnknownMethod(t *testing.T) {
	t.Parallel()

	sh, err := NewShell(t.TempDir())
	require.NoError(t, err)
	_, err = sh.Invoke(t.Context(), "run", json.RawMessage(`{"command":"echo"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown shell method")
}
