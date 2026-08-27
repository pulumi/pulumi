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

package neo

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// runNonInteractiveSession runs a --print or resume session with Ctrl+C wired
// the way the TUI's ESC is: the first interrupt posts a user_cancel so the agent
// stops the turn (which, via the echoed event, kills any running local tool);
// a second interrupt, or a cancel the service never accepts, tears the session
// down.
func runNonInteractiveSession(ctx context.Context, session *Session, stderr io.Writer) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	return runSessionWithInterrupt(ctx, session, interrupt, stderr)
}

// runSessionWithInterrupt is runNonInteractiveSession with the signal source
// injected so the two-stage interrupt can be unit-tested.
func runSessionWithInterrupt(
	ctx context.Context, session *Session, interrupt <-chan os.Signal, stderr io.Writer,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-interrupt:
		}
		fmt.Fprintln(stderr, "Cancelling... (press Ctrl+C again to exit immediately)")
		if !postCancelWithRetry(ctx, session, interrupt, stderr) {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
		case <-interrupt:
			cancel()
		}
	}()

	return session.Run(ctx)
}

// postCancelWithRetry posts a user_cancel under the same retry policy the TUI's
// cancelRetry uses (cancelMaxPostAttempts posts, userMessageRetryDelay backoff).
// It returns false when the cancel never landed or a second interrupt arrived
// while retrying, in which case the caller should exit instead.
func postCancelWithRetry(
	ctx context.Context, session *Session, interrupt <-chan os.Signal, stderr io.Writer,
) bool {
	for attempt := 1; ; attempt++ {
		postCtx, postCancel := context.WithTimeout(ctx, 10*time.Second)
		err := session.Client.PostNeoTaskUserEvent(
			postCtx, session.OrgName, session.TaskID, apitype.AgentUserEventCancel{Type: userEventUserCancel})
		postCancel()
		if err == nil {
			return true
		}
		if attempt == 1 {
			fmt.Fprintf(stderr, "warning: cancel not accepted yet, retrying: %v\n", err)
		}
		if attempt >= cancelMaxPostAttempts {
			fmt.Fprintf(stderr, "error: cancel not sent: %v\n", err)
			return false
		}
		t := time.NewTimer(userMessageRetryDelay(attempt))
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return false
		case <-interrupt:
			t.Stop()
			return false
		}
	}
}
