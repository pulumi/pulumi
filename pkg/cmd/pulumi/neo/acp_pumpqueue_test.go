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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
)

// finishAction builds a pumpAction whose turn result carries reason, used as a
// distinguishable payload in queue-ordering assertions.
func finishAction(reason acp.StopReason) pumpAction {
	return pumpAction{finish: &turnResult{reason: reason}}
}

func TestPumpQueueFIFO(t *testing.T) {
	t.Parallel()

	q := newPumpQueue()
	q.push(finishAction("a"))
	q.push(finishAction("b"))
	q.push(finishAction("c"))

	for _, want := range []acp.StopReason{"a", "b", "c"} {
		a, ok := q.pop()
		require.True(t, ok)
		require.NotNil(t, a.finish)
		assert.Equal(t, want, a.finish.reason)
	}
}

func TestPumpQueuePopBlocksUntilPush(t *testing.T) {
	t.Parallel()

	type popResult struct {
		a  pumpAction
		ok bool
	}
	q := newPumpQueue()
	got := make(chan popResult, 1)
	go func() {
		a, ok := q.pop()
		got <- popResult{a, ok}
	}()

	// The pop above must be blocked: nothing has been pushed yet.
	select {
	case <-got:
		t.Fatal("pop returned before anything was pushed")
	case <-time.After(20 * time.Millisecond):
	}

	q.push(finishAction("x"))
	select {
	case r := <-got:
		require.True(t, r.ok)
		assert.Equal(t, acp.StopReason("x"), r.a.finish.reason)
	case <-time.After(2 * time.Second):
		t.Fatal("pop did not observe the push")
	}
}

// TestPumpQueueCloseDeliversQueued is the teardown guarantee the pump relies
// on: close stops intake, but everything queued ahead of it — in particular the
// final turn result — is still delivered before pop reports done.
func TestPumpQueueCloseDeliversQueued(t *testing.T) {
	t.Parallel()

	q := newPumpQueue()
	q.push(finishAction("before1"))
	q.push(finishAction("before2"))
	q.close()
	q.push(finishAction("after"))

	a, ok := q.pop()
	require.True(t, ok)
	assert.Equal(t, acp.StopReason("before1"), a.finish.reason)
	a, ok = q.pop()
	require.True(t, ok)
	assert.Equal(t, acp.StopReason("before2"), a.finish.reason)

	_, ok = q.pop()
	assert.False(t, ok, "a push after close must be dropped and pop must report done")
}

func TestPumpQueueCloseUnblocksWaitingPop(t *testing.T) {
	t.Parallel()

	q := newPumpQueue()
	done := make(chan bool, 1)
	go func() {
		_, ok := q.pop()
		done <- ok
	}()

	q.close()
	select {
	case ok := <-done:
		assert.False(t, ok, "close must wake a blocked pop and report done")
	case <-time.After(2 * time.Second):
		t.Fatal("close did not unblock the waiting pop")
	}
}
