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

package ints

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// Regression test: a stack reference's passphrase prompt runs while the interactive display is up. The display used to
// read stdin at the same time, so every other keystroke was lost and the update hung.
func TestStackReferencePassphrasePromptDuringUpdate(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("The interactive tree display is not used on Windows")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory("stack_reference_passphrase")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.CWD = filepath.Join(e.RootPath, "root")
	e.Passphrase = "rootpass"
	e.RunCommand("pulumi", "stack", "init", "root")
	e.RunCommand("pulumi", "up", "--yes", "--skip-preview")

	e.CWD = filepath.Join(e.RootPath, "core")
	e.Passphrase = "corepass"
	e.RunCommand("pulumi", "stack", "init", "core")

	// Prompts only appear when interactive, which CI detection would turn off.
	e.NoPassphrase = true
	e.Env = append(e.Env, "PULUMI_DISABLE_CI_DETECTION=1")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	cmd := e.SetupCommandIn(ctx, e.CWD, "pulumi", "up", "--yes", "--skip-preview", "-s", "core")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	require.NoError(t, err)
	defer ptmx.Close()

	var m sync.Mutex
	var output bytes.Buffer
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			m.Lock()
			output.Write(buf[:n])
			m.Unlock()
			if err != nil {
				return
			}
		}
	}()
	snapshot := func() string {
		m.Lock()
		defer m.Unlock()
		return output.String()
	}

	const prompt = "Enter your passphrase to unlock config/secrets"
	// Typing one key at a time is what exposed the bug: each key went to whichever reader won.
	typeSlowly := func(text string) {
		for _, b := range []byte(text + "\r") {
			_, err := ptmx.Write([]byte{b})
			require.NoError(t, err)
			time.Sleep(50 * time.Millisecond)
		}
	}

	require.Eventually(t, func() bool { return strings.Contains(snapshot(), prompt) },
		time.Minute, 100*time.Millisecond, "core stack prompt")
	typeSlowly("corepass")

	// The core prompt is echoed again when it closes, so only count a prompt shown after the update starts.
	rootPromptShown := func() bool {
		_, afterUpdateStarts, _ := strings.Cut(snapshot(), "Updating (core)")
		return strings.Contains(afterUpdateStarts, prompt)
	}
	require.Eventually(t, rootPromptShown, time.Minute, 100*time.Millisecond, "root stack prompt")
	typeSlowly("rootpass")

	require.NoError(t, cmd.Wait(), "output: %s", snapshot())

	e.NoPassphrase = false
	stdout, _ := e.RunCommand("pulumi", "stack", "output", "rootPassword", "--show-secrets", "-s", "core")
	assert.Equal(t, "hunter2", strings.TrimSpace(stdout))
}
