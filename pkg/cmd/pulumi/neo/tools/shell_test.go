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

	sh := NewShell(t.TempDir())
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

	sh := NewShell(t.TempDir())
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

	sh := NewShell(t.TempDir())
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

	sh := NewShell(root)
	res, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"pwd","cwd":"`+sub+`"}`))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(res.(map[string]any)["stdout"].(string), sub))
}

func TestShell_ExecuteRejectsCwdOutsideRoot(t *testing.T) {
	t.Parallel()

	sh := NewShell(t.TempDir())
	_, err := sh.Invoke(t.Context(), "shell_execute",
		json.RawMessage(`{"command":"echo hi","cwd":"/tmp"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestShell_ExecuteRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	sh := NewShell(t.TempDir())
	_, err := sh.Invoke(t.Context(), "shell_execute", json.RawMessage(`{"command":""}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty command")
}

func TestShell_RejectsUnknownMethod(t *testing.T) {
	t.Parallel()

	sh := NewShell(t.TempDir())
	_, err := sh.Invoke(t.Context(), "run", json.RawMessage(`{"command":"echo"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown shell method")
}
