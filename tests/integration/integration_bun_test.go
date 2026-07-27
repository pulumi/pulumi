// Copyright 2016, Pulumi Corporation.
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
)

// Test that we can run a program, attach a debugger to it, and send debugging commands using the JSC Debugger Protocol
// and finally that the program terminates successfully after the debugger is detached.
func TestDebuggerAttachBun(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(filepath.Join("bun", "debugger"))

	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "install")
	// bun install (run by pulumi install) overwrites global links, so link after.
	e.RunCommand("bun", "link", "@pulumi/pulumi")

	e.Env = append(e.Env, "PULUMI_DEBUG_COMMANDS=true")
	e.RunCommand("pulumi", "stack", "init", "debugger-test")
	e.RunCommand("pulumi", "stack", "select", "debugger-test")

	eventLog := filepath.Join(e.RootPath, "debugger.log")
	cmd := e.SetupCommandIn(e.Context(), e.CWD, "pulumi", "preview", "--attach-debugger", "--event-log", eventLog)
	var previewStdout, previewStderr bytes.Buffer
	cmd.Stdout = &previewStdout
	cmd.Stderr = &previewStderr
	e.Logf("Running command %v %v", cmd.Path, strings.Join(cmd.Args[1:], " "))
	require.NoError(t, cmd.Start())

	var previewErr error
	previewDone := make(chan struct{})
	go func() {
		defer close(previewDone)
		previewErr = cmd.Wait()
	}()

	previewDiagnostics := func() string {
		var sb strings.Builder
		select {
		case <-previewDone:
			fmt.Fprintf(&sb, "pulumi preview exited early: %v\n", previewErr)
		default:
			if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
				fmt.Fprintf(&sb, "pulumi preview is still running; sending SIGQUIT to dump goroutine stacks failed: %v\n", err)
			} else {
				select {
				case <-previewDone:
					fmt.Fprintf(&sb, "pulumi preview was still running; goroutine stacks from SIGQUIT are in stderr below\n")
				case <-time.After(15 * time.Second):
					fmt.Fprintf(&sb, "pulumi preview was still running and did not exit within 15s of SIGQUIT\n")
				}
			}
		}
		select {
		case <-previewDone:
			fmt.Fprintf(&sb, "stdout:\n%s\nstderr:\n%s\n", previewStdout.String(), previewStderr.String())
		default:
		}
		if contents, err := os.ReadFile(eventLog); err != nil {
			fmt.Fprintf(&sb, "reading event log: %v", err)
		} else {
			fmt.Fprintf(&sb, "event log contents:\n%s", contents)
		}
		return sb.String()
	}

	// Wait for the debugging event
	wait := 20 * time.Millisecond
	var debugEvent *apitype.StartDebuggingEvent
outer:
	for range 50 {
		events, err := readUpdateEventLog(eventLog)
		require.NoError(t, err)
		for _, event := range events {
			if event.StartDebuggingEvent != nil {
				debugEvent = event.StartDebuggingEvent
				break outer
			}
		}
		time.Sleep(wait)
		if wait < 500*time.Millisecond {
			wait *= 2
		}
	}
	if debugEvent == nil {
		require.NotNilf(t, debugEvent, "no StartDebuggingEvent appeared in the event log; %s", previewDiagnostics())
	}

	wsURL, ok := debugEvent.Config["url"].(string)
	require.True(t, ok)
	require.NotEmpty(t, wsURL)

	// bun is launched with BUN_INSPECT=...?wait=1, which blocks the program from running until a
	// debug frontend sends Inspector.initialized. Send it to let the program run, then wait for
	// bun to acknowledge it before detaching: closing the WebSocket while bun is still waiting
	// leaves the program blocked indefinitely.
	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	require.NoError(t, err)
	require.NoError(t, ws.SetDeadline(time.Now().Add(30*time.Second)))
	require.NoError(t, websocket.Message.Send(ws, `{"id":1,"method":"Runtime.enable"}`))
	require.NoError(t, websocket.Message.Send(ws, `{"id":2,"method":"Inspector.initialized"}`))
	var received []string
	for {
		var msg string
		err := websocket.Message.Receive(ws, &msg)
		require.NoErrorf(t, err, "failed waiting for bun to acknowledge Inspector.initialized; "+
			"messages received so far:\n%s", strings.Join(received, "\n"))
		received = append(received, msg)
		if strings.Contains(msg, `"id":2`) {
			break
		}
	}
	require.NoError(t, ws.Close())

	// Verify the program completed successfully.
	select {
	case <-previewDone:
		require.NoError(t, previewErr, "pulumi preview failed:\nstdout:\n%s\nstderr:\n%s",
			previewStdout.String(), previewStderr.String())
	case <-time.After(60 * time.Second):
		require.FailNowf(t, "timed out waiting for program to complete after detaching the debugger",
			"bun likely never resumed execution.\ndebugger responses received:\n%s\n%s",
			strings.Join(received, "\n"), previewDiagnostics())
	}
}
