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

package fsa

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type FSA[Cursor any] struct {
	*fsa[Cursor]
}

func New[Cursor any]() FSA[Cursor] {
	return FSA[Cursor]{&fsa[Cursor]{
		cursors: map[cursorID]*cursor[Cursor]{},
		nodes:   map[nodeID]*node[Cursor]{},
		edges:   map[edgeID]ConditionFunc[Cursor]{},
	}}
}

// TODO[https://github.com/golang/go/issues/75757]: Should be a type alias
type NodeFunc[Cursor any] func(context.Context, FSA[Cursor], Edge, Cursor) error

type Node struct{ id nodeID }

// A condition guards an edge: returning ConditionPass grants the cursor the move along that edge.
//
// Conditions are sampled at most once per visit: after returning ConditionFail for a cursor, a condition
// is not asked again until that cursor re-enters the edge's source node. A cursor that stays put is never
// re-asked, so state changes a condition does not cause are not observed until the cursor's next visit.
// Every Progress call starts a fresh visit.
//
// TODO[https://github.com/golang/go/issues/75757]: Should be a type alias
type ConditionFunc[Cursor any] func(context.Context, FSA[Cursor], Cursor) (ConditionResult, error)

type ConditionResult struct{ kind int8 }

var (
	ConditionUnknown = ConditionResult{0 /* The zero value */} // Error
	ConditionPass    = ConditionResult{1}
	ConditionFail    = ConditionResult{2}
)

type Edge struct {
	id edgeID
}

func (fsa FSA[Cursor]) NewNode(f NodeFunc[Cursor]) Node {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	id := nodeID(fsa.idCounter)
	fsa.nodes[id] = &node[Cursor]{f, nil}
	return Node{id}
}

func (fsa FSA[Cursor]) NewEdge(f ConditionFunc[Cursor], from, to Node) Edge {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	id := edgeID(fsa.idCounter)
	fsa.edges[id] = f
	fsa.nodes[from.id].edges = append(fsa.nodes[from.id].edges,
		struct {
			e edgeID
			n nodeID
		}{id, to.id})

	// If we are mid-progress, settled cursors at from can now try the new edge. Their evaluated
	// watermark is left alone, so only the new edge is tried — edges already asked this visit are not
	// re-asked. Sorted order keeps the ready queue deterministic.
	if r := fsa.currentRun; r != nil {
		for _, cid := range slices.Sorted(maps.Keys(fsa.cursors)) {
			c := fsa.cursors[cid]
			if c.node == from.id && c.state == stateParked {
				c.state = stateReady
				r.ready = append(r.ready, cid)
			}
		}
		r.notify()
	}
	return Edge{id}
}

// Place a cursor on n.
//
// The entry function for n is not called.
func (fsa FSA[Cursor]) NewCursor(c Cursor, n Node) {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	id := cursorID(fsa.idCounter)
	fsa.cursors[id] = &cursor[Cursor]{node: n.id, c: c, state: stateIdle}

	// If we are mid-progress, make sure our cursor is progressed.
	if r := fsa.currentRun; r != nil {
		fsa.cursors[id].state = stateReady
		r.ready = append(r.ready, id)
		r.notify()
	}
}

// TODO[https://github.com/golang/go/issues/75757]: Should be a type alias
type Runner func(context.Context, func(context.Context)) error

// A runner that runs all processes sync.
var SyncRunner Runner = func(ctx context.Context, f func(context.Context)) error {
	f(ctx)
	return nil
}

// Iterate the FSA until no cursor can move, dispatching condition evaluations and node entry functions
// through runner.
//
// Cursors progress unordered, but a cursor arriving at an occupied node waits for each occupant to settle
// first. An occupant that can move away is *guaranteed* the chance to do so; an occupant that cannot
// (every outgoing edge was tried and failed since it arrived at its node, or the node has no outgoing
// edges) is overwritten by the arrival. Two cursors concurrently moving to the same node are a race;
// Progress reports it as an error.
//
// runner may run the function it is handed on another goroutine; Progress does not return until every
// dispatched function has completed. If runner returns an error, the function it was handed must never
// run.
func (fsa FSA[Cursor]) Progress(ctx context.Context, runner Runner) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	r := &run{
		wake:    make(chan struct{}, 1),
		claims:  map[nodeID]cursorID{},
		waiters: map[nodeID]cursorID{},
	}

	fsa.m.Lock()
	contract.Assertf(fsa.currentRun == nil, "we can only progress one run at a time")
	fsa.currentRun = r
	// Every Progress call starts a fresh visit for every cursor.
	for _, id := range slices.Sorted(maps.Keys(fsa.cursors)) {
		fsa.cursors[id].state = stateReady
		fsa.cursors[id].evaluated = 0
		r.ready = append(r.ready, id)
	}
	fsa.m.Unlock()
	defer func() {
		fsa.m.Lock()
		fsa.currentRun = nil
		fsa.m.Unlock()
	}()

	inFlight := 0 // Dispatched functions that have not yet delivered a completion. Loop-local.
	var errs []error
	fail := func(err error) {
		errs = append(errs, err)
		cancel(err) // Let outstanding callbacks bail early; the first failure is the cause
	}
	// Record a callback failure. Cancellation collateral (callbacks returning ctx's error because we or
	// the caller cancelled) is folded into the cancellation's cause rather than reported on its own.
	failResult := func(err error) {
		collateral := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		if collateral && context.Cause(ctx) != nil {
			if len(errs) > 0 {
				return // The cause is already recorded
			}
			err = context.Cause(ctx)
		}
		fail(err)
	}

	for {
		fsa.m.Lock()
		batch := r.ready
		results := r.results
		r.ready, r.results = nil, nil

		if len(batch) == 0 && len(results) == 0 {
			if inFlight > 0 {
				fsa.m.Unlock()
				if len(errs) > 0 {
					// Already failing: just wait for outstanding work to drain.
					<-r.wake
				} else {
					select {
					case <-r.wake:
					case <-ctx.Done():
						fail(context.Cause(ctx))
					}
				}
				continue
			}
			if len(errs) == 0 && fsa.commitStalledLocked(r) {
				fsa.m.Unlock()
				continue
			}
			fsa.m.Unlock()
			return errors.Join(errs...)
		}

		// All state transitions are decided here, under the lock, on this goroutine. The expensive
		// work (conditions, node entry functions) is collected and dispatched after the lock is
		// released; each dispatched function delivers a completion back to this loop as its last act.
		var dispatches []func(context.Context)

		for _, res := range results {
			inFlight--
			switch res.kind {
			case completionErr:
				failResult(res.err)
			case completionPassed:
				if len(errs) > 0 {
					continue
				}
				if holder, taken := r.claims[res.target]; taken {
					fail(fmt.Errorf("race: cursors %d and %d are both moving to node %d",
						holder, res.cursor, res.target))
					continue
				}
				r.claims[res.target] = res.cursor
				cur := fsa.cursors[res.cursor]
				cur.state = stateMovePending
				// Entry is now inevitable (each occupant of the target either moves away or is
				// overwritten), so run the entry function eagerly; only the commit may wait.
				enter := fsa.nodes[res.target].f
				id, edge, target, data := res.cursor, res.edge, res.target, cur.c
				inFlight++
				dispatches = append(dispatches, func(ctx context.Context) {
					err := enter(ctx, fsa, Edge{edge}, data)
					fsa.deliver(r, completion{kind: completionMoved, cursor: id, target: target, err: err})
				})
			case completionParked:
				if len(errs) > 0 {
					continue
				}
				cur := fsa.cursors[res.cursor]
				cur.evaluated = res.endLen
				if len(fsa.nodes[cur.node].edges) > res.endLen {
					// Edges were appended while we evaluated: try just the new ones.
					cur.state = stateReady
					r.ready = append(r.ready, res.cursor)
					continue
				}
				cur.state = stateParked
				if w, ok := r.waiters[cur.node]; ok {
					// An arrival is waiting on our node and we are provably stuck.
					delete(r.waiters, cur.node)
					fsa.attemptCommitLocked(r, w, cur.node)
				}
			case completionMoved:
				if res.err != nil {
					failResult(res.err)
					continue
				}
				if len(errs) > 0 {
					continue
				}
				fsa.attemptCommitLocked(r, res.cursor, res.target)
			}
		}

		for _, id := range batch {
			if len(errs) > 0 {
				continue // Failing: don't start new work
			}
			cur := fsa.cursors[id]
			edges := fsa.nodes[cur.node].edges
			if len(edges) <= cur.evaluated {
				cur.state = stateParked // Settled: nothing untried this visit
				if w, ok := r.waiters[cur.node]; ok {
					delete(r.waiters, cur.node)
					fsa.attemptCommitLocked(r, w, cur.node)
				}
				continue
			}
			suffix := edges[cur.evaluated:] // Earlier edges were already asked this visit
			conds := make([]condRef[Cursor], len(suffix))
			for i, e := range suffix {
				conds[i] = condRef[Cursor]{fsa.edges[e.e], e.e, e.n}
			}
			cur.state = stateEvaluating
			endLen := len(edges)
			data := cur.c
			inFlight++
			dispatches = append(dispatches, func(ctx context.Context) {
				for _, c := range conds {
					result, err := c.fn(ctx, fsa, data)
					if err != nil {
						fsa.deliver(r, completion{kind: completionErr, cursor: id, err: err})
						return
					}
					switch result {
					case ConditionPass:
						fsa.deliver(r, completion{kind: completionPassed, cursor: id, edge: c.e, target: c.to})
						return
					case ConditionFail:
						// Not this condition this time
					default:
						fsa.deliver(r, completion{
							kind: completionErr, cursor: id,
							err: errors.New("condition returned unknown"),
						})
						return
					}
				}
				fsa.deliver(r, completion{kind: completionParked, cursor: id, endLen: endLen})
			})
		}
		fsa.m.Unlock()

		for _, d := range dispatches {
			if len(errs) > 0 {
				inFlight-- // Failing: drop work that has not started
				continue
			}
			if err := runner(ctx, d); err != nil {
				inFlight--
				fail(err)
			}
		}
	}
}

// Hand a completion from a dispatched function back to the Progress loop.
func (f *fsa[Cursor]) deliver(r *run, c completion) {
	f.m.Lock()
	r.results = append(r.results, c)
	f.m.Unlock()
	r.notify()
}

// Commit id's granted move into target if every occupant of target has settled, overwriting occupants that
// cannot move away; otherwise register id to wait for the occupants to settle.
func (f *fsa[Cursor]) attemptCommitLocked(r *run, id cursorID, target nodeID) {
	for oid, o := range f.cursors {
		if oid == id || o.node != target {
			continue
		}
		if o.state != stateParked {
			// The occupant may still move away; it is guaranteed the chance to do so.
			cur := f.cursors[id]
			cur.state = stateDeferred
			cur.target = target
			r.waiters[target] = id
			return
		}
	}
	var doomed []cursorID
	for oid, o := range f.cursors {
		if oid != id && o.node == target {
			doomed = append(doomed, oid) // Overwrite: the occupant cannot move away
		}
	}
	for _, oid := range doomed {
		delete(f.cursors, oid)
	}
	f.commitLocked(r, id, target)
}

func (f *fsa[Cursor]) commitLocked(r *run, id cursorID, target nodeID) {
	cur := f.cursors[id]
	vacated := cur.node
	cur.node = target
	cur.state = stateReady
	cur.evaluated = 0 // Entering a node starts a fresh visit
	delete(r.claims, target)
	r.ready = append(r.ready, id)
	if w, ok := r.waiters[vacated]; ok {
		// Vacating our old node may unblock an arrival that was waiting on us.
		delete(r.waiters, vacated)
		f.attemptCommitLocked(r, w, vacated)
	}
}

// Resolve a stall: no work is queued or in flight, but deferred cursors remain, each waiting on another
// deferred cursor's node (a cycle). A deferred cursor's move is inevitable, so commit them all
// simultaneously. Reports whether anything moved.
func (f *fsa[Cursor]) commitStalledLocked(r *run) bool {
	var movers []cursorID
	targets := map[nodeID]bool{}
	for id, c := range f.cursors {
		if c.state == stateDeferred {
			movers = append(movers, id)
			targets[c.target] = true
		}
	}
	if len(movers) == 0 {
		return false
	}
	var doomed []cursorID
	for id, c := range f.cursors {
		if c.state == stateParked && targets[c.node] {
			doomed = append(doomed, id) // Overwrite: a stuck occupant of an entered node
		}
	}
	for _, id := range doomed {
		delete(f.cursors, id)
	}
	slices.Sort(movers)
	for _, id := range movers {
		c := f.cursors[id]
		c.node = c.target
		c.state = stateReady
		c.evaluated = 0
		r.ready = append(r.ready, id)
	}
	clear(r.claims)
	clear(r.waiters)
	return true
}

// The subset of cursors that are parked
func (fsa FSA[Cursor]) Parked(yield func(Cursor, Node) bool) {
	fsa.cursorsInner(yield, true)
}

// The list of cursors and the node they are on
func (fsa FSA[Cursor]) Cursors(yield func(Cursor, Node) bool) {
	fsa.cursorsInner(yield, false)
}

func (fsa FSA[Cursor]) GetEdge(e Edge) (from, to Node) {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	for nID, n := range fsa.nodes {
		for _, pE := range n.edges {
			if pE.e == e.id {
				return Node{nID}, Node{pE.n}
			}
		}
	}
	panic("Edge was not built from this graph")
}

func (fsa FSA[Cursor]) cursorsInner(yield func(Cursor, Node) bool, onlyParked bool) {
	fsa.m.Lock()
	type entry struct {
		c Cursor
		n Node
	}
	var snapshot []entry
	for _, id := range slices.Sorted(maps.Keys(fsa.cursors)) {
		c := fsa.cursors[id]
		// A settled cursor on a node with no outgoing edges is resting, not blocked: not parked.
		if onlyParked && (c.state != stateParked || len(fsa.nodes[c.node].edges) == 0) {
			continue
		}
		snapshot = append(snapshot, entry{c.c, Node{c.node}})
	}
	fsa.m.Unlock()
	for _, e := range snapshot {
		if !yield(e.c, e.n) {
			return
		}
	}
}

// The core of the FSA
type fsa[Cursor any] struct {
	m sync.Mutex

	idCounter uint64

	cursors map[cursorID]*cursor[Cursor]
	nodes   map[nodeID]*node[Cursor]
	edges   map[edgeID]ConditionFunc[Cursor]

	currentRun *run
}

// The mutable state of a single Progress call. Guarded by fsa.m, except wake.
type run struct {
	ready   []cursorID          // Cursors queued for condition evaluation
	results []completion        // Completions delivered by dispatched functions
	wake    chan struct{}       // Cap 1; poked after each delivery
	claims  map[nodeID]cursorID // Granted entries not yet committed: target -> arriving cursor
	waiters map[nodeID]cursorID // Deferred arrivals: contested node -> waiting cursor
}

func (r *run) notify() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

type completion struct {
	kind   completionKind
	cursor cursorID
	edge   edgeID // completionPassed
	target nodeID // completionPassed, completionMoved
	endLen int    // completionParked: length of the node's edge list when evaluation began
	err    error  // completionErr, completionMoved
}

type completionKind uint8

const (
	completionPassed completionKind = iota // A condition passed: cursor may enter target via edge
	completionParked                       // Every condition failed
	completionMoved                        // The node entry function finished
	completionErr                          // A callback failed
)

type condRef[Cursor any] struct {
	fn ConditionFunc[Cursor]
	e  edgeID
	to nodeID
}

type node[Cursor any] struct {
	f     NodeFunc[Cursor]
	edges []struct {
		e edgeID
		n nodeID
	}
}

type cursor[Cursor any] struct {
	node   nodeID
	c      Cursor
	state  cursorState
	target nodeID // Valid when state == stateDeferred
	// How many leading edges of node have been asked (and failed) during this visit. Edge lists are
	// append-only, so a re-queued cursor evaluates only the suffix beyond this watermark. It resets when
	// the cursor enters a node — and only then: conditions are sampled at most once per visit.
	evaluated int
}

type cursorState uint8

const (
	stateIdle        cursorState = iota // Not part of an active run
	stateReady                          // Queued for condition evaluation
	stateEvaluating                     // Condition evaluation in flight
	stateMovePending                    // Node entry function in flight; the target is claimed
	stateDeferred                       // Entry granted; waiting for the target's occupants to settle
	stateParked                         // Settled: every outgoing edge (possibly none) was asked this visit
)

type (
	nodeID   uint64 // A UUID for the node
	edgeID   uint64 // A UUID for the edge
	cursorID uint64 // A UUID for the cursor
)
