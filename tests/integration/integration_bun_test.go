// Copyright 2016-2022, Pulumi Corporation.
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
	"path/filepath"
	"strings"
	"sync"
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

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.RunCommand("pulumi", "preview", "--attach-debugger",
			"--event-log", filepath.Join(e.RootPath, "debugger.log"))
	}()

	// Wait for the debugging event
	wait := 20 * time.Millisecond
	var debugEvent *apitype.StartDebuggingEvent
outer:
	for i := 0; i < 50; i++ {
		events, err := readUpdateEventLog(filepath.Join(e.RootPath, "debugger.log"))
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
	require.NotNil(t, debugEvent)

	wsURL, ok := debugEvent.Config["url"].(string)
	require.True(t, ok)
	require.NotEmpty(t, wsURL)

	// Use the JavaScriptCore Debug Protocol to resume the paused program.
	ws, err := websocket.Dial(wsURL, "", "http://localhost")
	require.NoError(t, err)
	require.NoError(t, ws.SetDeadline(time.Now().Add(30*time.Second)))
	require.NoError(t, websocket.Message.Send(ws, `{"id":1,"method":"Runtime.enable"}`))
	require.NoError(t, websocket.Message.Send(ws, `{"id":2,"method":"Inspector.initialized"}`))
	require.NoError(t, websocket.Message.Send(ws, `{"id":3,"method":"Debugger.resume"}`))
	// Wait for bun to acknowledge Debugger.resume before closing. If we close the WebSocket
	// before bun processes the resume, the program can remain paused indefinitely.
	for {
		var msg string
		require.NoError(t, websocket.Message.Receive(ws, &msg))
		if strings.Contains(msg, `"id":3`) {
			break
		}
	}
	require.NoError(t, ws.Close())

	// Verify the program completed successfully.
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(60 * time.Second):
		t.Fatal("timed out waiting for program to complete")
	}
}
