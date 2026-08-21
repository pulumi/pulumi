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
	"cmp"
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

func New[Cursor any](opts ...Option[Cursor]) FSA[Cursor] {
	f := &fsa[Cursor]{
		cursors: map[cursorID]*cursor[Cursor]{},
		nodes:   map[nodeID]*node[Cursor]{},
		edges:   map[edgeID]ConditionFunc[Cursor]{},
	}
	for _, o := range opts {
		o(f)
	}
	return FSA[Cursor]{f}
}

type Option[Cursor any] func(*fsa[Cursor])

// ReplaceFunc is told when replaced, an occupant of at that could not move away, is overwritten by the
// arrival of by. It is called on the Progress goroutine without the machine's lock held, after by's
// entry function has completed and before any further work is dispatched, so it may call back into
// the machine.
//
// TODO[https://github.com/golang/go/issues/75757]: Should be a type alias
type ReplaceFunc[Cursor any] func(replaced, by Cursor, at Node)

// OnReplace registers f to be told about every overwritten cursor. Overwriting is the only way a
// cursor is removed from the machine, so f sees every cursor's end.
func OnReplace[Cursor any](f ReplaceFunc[Cursor]) Option[Cursor] {
	return func(m *fsa[Cursor]) { m.onReplace = f }
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

// CursorRef identifies a cursor placed by NewCursor for as long as it is in the machine.
type CursorRef struct{ id cursorID }

// Place a cursor on n. The entry function for n is not called.
//
// Placing is an arrival: an occupant of n that is settled this visit is overwritten at once, and any
// other occupant is overwritten as soon as it settles without having moved away (see [FSA.Progress]).
func (fsa FSA[Cursor]) NewCursor(c Cursor, n Node) CursorRef {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	fsa.idCounter++
	id := cursorID(fsa.idCounter)
	fsa.arrivals++
	fsa.cursors[id] = &cursor[Cursor]{node: n.id, c: c, state: stateIdle, arrived: fsa.arrivals}

	// If we are mid-progress, make sure our cursor is progressed.
	if r := fsa.currentRun; r != nil {
		var stuck []cursorID
		for oid, o := range fsa.cursors {
			if oid != id && o.node == n.id && o.state == stateParked {
				stuck = append(stuck, oid) // Settled this visit: it cannot move away
			}
		}
		slices.Sort(stuck)
		for _, oid := range stuck {
			fsa.removeLocked(r, oid, id)
		}
		fsa.cursors[id].state = stateReady
		r.ready = append(r.ready, id)
		r.notify()
	}
	return CursorRef{id}
}

// Remove takes cursors out of the machine, all or none: if any of them is not in the machine it removes
// nothing and reports false. A removed cursor simply ceases — it is not an overwrite, so OnReplace is
// not told. Remove may be called from callbacks; a removed cursor's in-flight condition or entry
// function is waited for but its result is discarded, which is the one exception to an entry function
// that returns nil being seen as committed.
func (fsa FSA[Cursor]) Remove(refs ...CursorRef) bool {
	fsa.m.Lock()
	defer fsa.m.Unlock()
	for _, ref := range refs {
		if _, ok := fsa.cursors[ref.id]; !ok {
			return false
		}
	}
	for _, ref := range refs {
		delete(fsa.cursors, ref.id)
		if r := fsa.currentRun; r != nil {
			maps.DeleteFunc(r.claims, func(_ nodeID, id cursorID) bool { return id == ref.id })
			maps.DeleteFunc(r.waiters, func(_ nodeID, id cursorID) bool { return id == ref.id })
		}
	}
	return true
}

type Runner = func(context.Context, func(context.Context)) error

// A runner that runs all processes sync.
func SyncRunner(ctx context.Context, f func(context.Context)) error {
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
// dispatched function has delivered its result, which is the last thing such a function does before
// returning. If runner returns an error, the function it was handed must never run.
//
// A failure (a callback error, a race, or ctx being cancelled) aborts the run: no further conditions or
// entry functions are started, ctx is cancelled so that outstanding callbacks may bail early, and
// Progress waits for them. Their results still count: a node entry function that returns nil is
// *guaranteed* to be seen as committed — its cursor is at the node it entered when Progress returns.
//
// Cursors on one node are ordered by arrival, and a cursor that settles on a node holding a later
// arrival is overwritten by the latest one. Usually that is decided the instant the arrival commits,
// because an arrival waits for the occupants to settle. Two cases leave an unsettled occupant beside an
// arrival instead, keeping the occupant's chance to move: an entry committed while the run is failing
// (the abort denied the occupant its chance this run), and a placement by [FSA.NewCursor]. Such an
// occupant is overwritten as soon as it settles without having moved away, which a later Progress may
// decide.
func (fsa FSA[Cursor]) Progress(ctx context.Context, runner Runner) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	r := &run[Cursor]{
		wake:    make(chan struct{}, 1),
		abort:   cancel,
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
	// The run is failing once an error is recorded or ctx is cancelled — by the caller, or by a callback
	// failure delivered but not yet processed. No further work is started from then on.
	failing := func() bool { return len(errs) > 0 || ctx.Err() != nil }
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
		if len(errs) > 0 && errors.Is(context.Cause(ctx), err) {
			return // A delivery aborted the run with this error before we processed it; already recorded
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
			if len(errs) == 0 && ctx.Err() != nil {
				fail(context.Cause(ctx)) // Cancelled before any callback could notice
			}
			if fsa.commitDeferredLocked(r) {
				replaced := r.takeReplaced()
				fsa.m.Unlock()
				fsa.reportReplaced(replaced)
				continue
			}
			// Ended under the same lock as the emptiness check: a mutation from here on is not part
			// of this run and waits for the next Progress.
			fsa.currentRun = nil
			fsa.m.Unlock()
			return errors.Join(errs...)
		}

		// All state transitions are decided here, under the lock, on this goroutine. The expensive
		// work (conditions, node entry functions) is collected and dispatched after the lock is
		// released; each dispatched function delivers a completion back to this loop as its last act.
		var dispatches []func(context.Context)

		for _, res := range results {
			inFlight--
			if _, present := fsa.cursors[res.cursor]; !present && res.kind != completionErr {
				continue // Removed while its work was in flight: the result is discarded
			}
			switch res.kind {
			case completionErr:
				failResult(res.err)
			case completionPassed:
				if failing() {
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
					// The runner may start us after the run failed: start nothing then.
					err := ctx.Err()
					if err == nil {
						err = enter(ctx, fsa, Edge{edge}, data)
					}
					fsa.deliver(r, completion{kind: completionMoved, cursor: id, target: target, err: err})
				})
			case completionParked:
				cur := fsa.cursors[res.cursor]
				cur.evaluated = res.endLen
				if len(fsa.nodes[cur.node].edges) > res.endLen {
					// Edges were appended while we evaluated: try just the new ones — unless we are
					// failing, in which case the cursor stays unsettled until the next Progress.
					if !failing() {
						cur.state = stateReady
						r.ready = append(r.ready, res.cursor)
					}
					continue
				}
				fsa.parkLocked(r, res.cursor)
			case completionMoved:
				if res.err != nil {
					failResult(res.err)
					continue
				}
				// Committed even when failing: the entry function ran and succeeded.
				fsa.attemptCommitLocked(r, res.cursor, res.target)
			}
		}

		for _, id := range batch {
			if failing() {
				continue // Failing: don't start new work
			}
			cur, present := fsa.cursors[id]
			if !present {
				continue // Removed since it was queued
			}
			edges := fsa.nodes[cur.node].edges
			if len(edges) <= cur.evaluated {
				fsa.parkLocked(r, id) // Settled: nothing untried this visit
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
					if err := ctx.Err(); err != nil {
						// The run failed (or the runner started us after it did): start nothing more.
						fsa.deliver(r, completion{kind: completionErr, cursor: id, err: err})
						return
					}
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
		replaced := r.takeReplaced()
		fsa.m.Unlock()
		fsa.reportReplaced(replaced)

		for _, d := range dispatches {
			if failing() {
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

func (f *fsa[Cursor]) reportReplaced(replaced []replacement[Cursor]) {
	if f.onReplace == nil {
		return
	}
	for _, rp := range replaced {
		f.onReplace(rp.replaced, rp.by, Node{rp.at})
	}
}

// Hand a completion from a dispatched function back to the Progress loop. A failure aborts the run at
// once, so that no dispatch queued behind it is started; the loop records it when it processes the
// completion.
func (f *fsa[Cursor]) deliver(r *run[Cursor], c completion) {
	if c.err != nil {
		r.abort(c.err)
	}
	f.m.Lock()
	r.results = append(r.results, c)
	f.m.Unlock()
	r.notify()
}

// Commit id's granted move into target if every occupant of target has settled, overwriting occupants that
// cannot move away; otherwise register id to wait for the occupants to settle.
func (f *fsa[Cursor]) attemptCommitLocked(r *run[Cursor], id cursorID, target nodeID) {
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
	slices.Sort(doomed)
	for _, oid := range doomed {
		f.removeLocked(r, oid, id)
	}
	f.commitLocked(r, id, target)
}

// Overwrite doomed with by, which is entering doomed's node.
func (f *fsa[Cursor]) removeLocked(r *run[Cursor], doomed, by cursorID) {
	d := f.cursors[doomed]
	if f.onReplace != nil {
		r.replaced = append(r.replaced, replacement[Cursor]{replaced: d.c, by: f.cursors[by].c, at: d.node})
	}
	delete(f.cursors, doomed)
}

// Settle id at its node: every outgoing edge (possibly none) has been asked this visit. A cursor that
// settles beside a later arrival is overwritten by the latest one; an arrival waiting on the node may
// now commit.
func (f *fsa[Cursor]) parkLocked(r *run[Cursor], id cursorID) {
	cur := f.cursors[id]
	cur.state = stateParked
	var latest cursorID
	for oid, o := range f.cursors {
		if o.node == cur.node && o.arrived > cur.arrived && (latest == 0 || o.arrived > f.cursors[latest].arrived) {
			latest = oid
		}
	}
	if latest != 0 {
		f.removeLocked(r, id, latest)
	}
	if w, ok := r.waiters[cur.node]; ok {
		delete(r.waiters, cur.node)
		f.attemptCommitLocked(r, w, cur.node)
	}
}

func (f *fsa[Cursor]) commitLocked(r *run[Cursor], id cursorID, target nodeID) {
	cur := f.cursors[id]
	vacated := cur.node
	cur.node = target
	cur.state = stateReady
	cur.evaluated = 0 // Entering a node starts a fresh visit
	f.arrivals++
	cur.arrived = f.arrivals
	delete(r.claims, target)
	r.ready = append(r.ready, id)
	if w, ok := r.waiters[vacated]; ok {
		// Vacating our old node may unblock an arrival that was waiting on us.
		delete(r.waiters, vacated)
		f.attemptCommitLocked(r, w, vacated)
	}
}

// Commit every deferred cursor at once. Called when nothing else can make progress: either the run is
// quiescent and the deferred cursors wait on each other's nodes (a cycle), or the run is failing and no
// further work will be started. A deferred cursor's entry function has completed, so its move is
// inevitable. Occupants of entered nodes that have settled are overwritten; occupants that have not —
// possible only when failing, since the abort denied them their chance this run — stay beside the
// arrival, to be overwritten once they settle without moving away. Reports whether anything moved.
func (f *fsa[Cursor]) commitDeferredLocked(r *run[Cursor]) bool {
	var movers []cursorID
	entering := map[nodeID]cursorID{} // A deferred cursor's target is claimed, so one mover per node
	for id, c := range f.cursors {
		if c.state == stateDeferred {
			movers = append(movers, id)
			entering[c.target] = id
		}
	}
	if len(movers) == 0 {
		return false
	}
	var doomed []cursorID
	for id, c := range f.cursors {
		if _, entered := entering[c.node]; !entered || c.state == stateDeferred {
			continue // Not in the way, or a mover that vacates
		}
		if c.state == stateParked {
			doomed = append(doomed, id) // Overwrite: a stuck occupant of an entered node
		}
	}
	slices.Sort(doomed)
	for _, id := range doomed {
		f.removeLocked(r, id, entering[f.cursors[id].node])
	}
	slices.Sort(movers)
	for _, id := range movers {
		c := f.cursors[id]
		c.node = c.target
		c.state = stateReady
		c.evaluated = 0
		f.arrivals++
		c.arrived = f.arrivals
		r.ready = append(r.ready, id)
	}
	clear(r.claims)
	clear(r.waiters)
	return true
}

// The subset of cursors that are parked, in arrival order
func (fsa FSA[Cursor]) Parked(yield func(Cursor, Node) bool) {
	fsa.cursorsInner(yield, true)
}

// The cursors and the node each is on, in arrival order: the order in which they were placed or last
// entered a node.
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
	byArrival := slices.SortedFunc(maps.Keys(fsa.cursors), func(a, b cursorID) int {
		return cmp.Compare(fsa.cursors[a].arrived, fsa.cursors[b].arrived)
	})
	for _, id := range byArrival {
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
	arrivals  uint64 // Arrival sequence, stamped onto cursors as they are placed or enter a node

	cursors map[cursorID]*cursor[Cursor]
	nodes   map[nodeID]*node[Cursor]
	edges   map[edgeID]ConditionFunc[Cursor]

	onReplace  ReplaceFunc[Cursor] // May be nil
	currentRun *run[Cursor]
}

// The mutable state of a single Progress call. Guarded by fsa.m, except wake.
type run[Cursor any] struct {
	ready   []cursorID          // Cursors queued for condition evaluation
	results []completion        // Completions delivered by dispatched functions
	wake    chan struct{}       // Cap 1; poked after each delivery
	abort   func(error)         // Cancels the run's context with the given cause
	claims  map[nodeID]cursorID // Granted entries not yet committed: target -> arriving cursor
	waiters map[nodeID]cursorID // Deferred arrivals: contested node -> waiting cursor
	// Overwrites decided under the lock, reported to onReplace once it is released.
	replaced []replacement[Cursor]
}

type replacement[Cursor any] struct {
	replaced, by Cursor
	at           nodeID
}

func (r *run[Cursor]) takeReplaced() []replacement[Cursor] {
	rp := r.replaced
	r.replaced = nil
	return rp
}

func (r *run[Cursor]) notify() {
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
	// When the cursor arrived at node (placed, or last committed), as a machine-wide sequence number.
	// A cursor that settles beside a later arrival is overwritten by the latest one.
	arrived uint64
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
