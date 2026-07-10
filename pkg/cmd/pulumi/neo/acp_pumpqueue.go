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
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
)

// pumpAction is one ordered step the writer performs: emit a session/update
// notification, launch a permission request, or resolve the active turn. Exactly
// one field is set. Routing all three through a single ordered queue keeps them
// in the order the agent produced them — in particular a turn resolves only
// after the updates that preceded it have been written — while letting the pump
// drain UIEvents without blocking on the editor.
type pumpAction struct {
	notify   acp.SessionUpdate  //nolint:unused // consumed by the ACP adapter, next in this stack
	approval *UIApprovalRequest //nolint:unused // consumed by the ACP adapter, next in this stack
	finish   *turnResult
}

// turnResult is how the event pump signals the waiting Prompt call that the
// current turn ended: with a stop reason, or a fatal error.
type turnResult struct {
	reason acp.StopReason
	err    error //nolint:unused // consumed by the ACP adapter, next in this stack
}

// pumpQueue is an unbounded, ordered, single-consumer queue of pumpActions. The
// pump pushes without ever blocking on the editor; drainPumpQueue pops in FIFO
// order. close stops intake, but actions already queued are still delivered —
// in particular the turn result the pump queues on teardown, so a waiting
// Prompt always resolves.
type pumpQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []pumpAction
	closed bool
}

func newPumpQueue() *pumpQueue {
	q := &pumpQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *pumpQueue) push(a pumpAction) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.items = append(q.items, a)
	q.cond.Signal()
}

// pop blocks until an action is available, returning ok=false once the queue is
// closed and everything queued before the close has been delivered.
func (q *pumpQueue) pop() (pumpAction, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return pumpAction{}, false
	}
	a := q.items[0]
	q.items = q.items[1:]
	return a, true
}

// close stops intake: later pushes are dropped, and pop reports done once the
// already-queued actions have been delivered.
func (q *pumpQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}
