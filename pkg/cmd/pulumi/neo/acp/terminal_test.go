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

package acp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedCaller answers the terminal/* methods with canned results and records
// the order they were called in.
type scriptedCaller struct {
	methods   []string
	exitCode  int
	signal    string
	output    string
	truncated bool
	waitErr   error
}

func (c *scriptedCaller) Call(_ context.Context, method string, _, result any) error {
	c.methods = append(c.methods, method)
	switch method {
	case "terminal/create":
		if r, ok := result.(*createTerminalResult); ok {
			r.TerminalID = "term_1"
		}
	case "terminal/wait_for_exit":
		if c.waitErr != nil {
			return c.waitErr
		}
		if r, ok := result.(*exitStatus); ok {
			// A signal-terminated process reports a null exit code (no ExitCode)
			// alongside the signal, matching the editor's wire shape.
			if c.signal != "" {
				r.Signal = c.signal
			} else {
				ec := c.exitCode
				r.ExitCode = &ec
			}
		}
	case "terminal/output":
		if r, ok := result.(*terminalOutputResult); ok {
			r.Output = c.output
			r.Truncated = c.truncated
		}
	}
	return nil
}

func TestClientTerminalRun(t *testing.T) {
	t.Parallel()

	sc := &scriptedCaller{exitCode: 0, output: "hello\n"}
	ct := &ClientTerminal{Caller: sc, SessionID: "sess_1"}

	res, err := ct.Run(t.Context(), "sh", []string{"-c", "echo hello"}, "/work", 1024)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", res.Output)
	assert.Equal(t, 0, res.ExitCode)
	assert.False(t, res.TimedOut)

	assert.Equal(t,
		[]string{"terminal/create", "terminal/wait_for_exit", "terminal/output", "terminal/release"},
		sc.methods)
}

func TestClientTerminalRunReportsExitCode(t *testing.T) {
	t.Parallel()

	sc := &scriptedCaller{exitCode: 2, output: "boom"}
	ct := &ClientTerminal{Caller: sc, SessionID: "sess_1"}

	res, err := ct.Run(t.Context(), "sh", []string{"-c", "exit 2"}, "/work", 0)
	require.NoError(t, err)
	assert.Equal(t, 2, res.ExitCode)
	assert.Equal(t, "boom", res.Output)
}

func TestClientTerminalRunKillsOnWaitError(t *testing.T) {
	t.Parallel()

	sc := &scriptedCaller{waitErr: context.DeadlineExceeded, output: "partial"}
	ct := &ClientTerminal{Caller: sc, SessionID: "sess_1"}

	res, err := ct.Run(t.Context(), "sh", []string{"-c", "sleep 100"}, "/work", 0)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.True(t, res.TimedOut)
	assert.Equal(t, "partial", res.Output, "partial output is collected after the kill")
	assert.Contains(t, sc.methods, "terminal/kill")
	assert.Contains(t, sc.methods, "terminal/release")
}

func TestClientTerminalRunWaitErrorNotMislabeledAsTimeout(t *testing.T) {
	t.Parallel()

	// A non-deadline wait failure (e.g. the editor's terminal went away) must
	// not be reported as a timeout; only a real deadline sets TimedOut.
	sc := &scriptedCaller{waitErr: errors.New("terminal vanished"), output: "partial"}
	ct := &ClientTerminal{Caller: sc, SessionID: "sess_1"}

	res, err := ct.Run(t.Context(), "sh", []string{"-c", "sleep 100"}, "/work", 0)
	require.Error(t, err)
	assert.False(t, res.TimedOut, "transport error must not be labeled a timeout")
	assert.Contains(t, sc.methods, "terminal/kill")
}

func TestClientTerminalRunReportsSignal(t *testing.T) {
	t.Parallel()

	// A signal-terminated process reports a null exit code; Signal carries the
	// cause and ExitCode decodes to 0 at this layer.
	sc := &scriptedCaller{signal: "SIGKILL", output: "killed"}
	ct := &ClientTerminal{Caller: sc, SessionID: "sess_1"}

	res, err := ct.Run(t.Context(), "sh", []string{"-c", "sleep 100"}, "/work", 0)
	require.NoError(t, err)
	assert.Equal(t, "SIGKILL", res.Signal)
}

func TestClientTerminalRunPropagatesCreateError(t *testing.T) {
	t.Parallel()

	rc := &recordingCaller{err: errors.New("no terminal")}
	ct := &ClientTerminal{Caller: rc, SessionID: "sess_1"}
	_, err := ct.Run(t.Context(), "sh", []string{"-c", "true"}, "/work", 0)
	require.Error(t, err)
	// No terminal was created, so there is nothing to kill or release: the
	// failed create must be the only call issued.
	assert.Equal(t, 1, rc.calls)
	assert.Equal(t, "terminal/create", rc.method)
}
