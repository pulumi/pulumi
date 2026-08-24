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

// Package workflow provides an augmented view on top of [fsa] with richer semantics: nodes have string
// identities and pass property maps along edges, edges compose (and, or, join), progress is reported as a
// stream of [WorkflowUpdate] events, and cursor placement can be saved with [Workflow.State] and restored
// with [FromState].
//
// A cursor carries values. It enters a node with the values the node it left produced, overlaid with
// whatever the edge it crossed set; those entered values never change while it sits there. The node's
// function turns them into the values the cursor leaves with, and [Workflow.Reconcile] re-runs it from
// the same entered values for cursors that did not move.
//
// The semantics of [fsa] leak through. Edge conditions are sampled at most once per visit, so an edge that
// depends on external state is not re-asked until the next [Workflow.Progress]; run Progress in a loop. A
// node's outgoing edges are asked in definition order and the first to pass is taken. A cursor arriving
// at an occupied node overwrites an occupant that cannot move, and two cursors moving to the same node
// in the same step is an error. A node function's error aborts the Progress: nothing further is
// started, but node functions already running are waited for, and every node function that returns nil
// has its cursor committed to that node — a successful deploy is never re-run. A cursor whose move had
// not yet been decided stays where it was and moves on the next Progress.
//
// Every cursor's life is reported: it begins with [CursorAdded] or as the New cursor of a [CursorsJoined],
// and ends with exactly one of [CursorReplaced] or [CursorsJoined].
package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type Workflow struct{ *workflow }

type workflow struct {
	g fsa.FSA[cursor]

	m          sync.Mutex
	nodes      map[string]fsa.Node
	nodeIDs    map[fsa.Node]string
	funcs      map[string]NodeFunc
	defined    []string        // Node IDs in definition order
	touched    map[string]bool // Nodes whose function ran during the current Progress
	joins      map[string]bool // Join edge names, which must be unique
	states     map[string]NodeState
	nextCursor int
	// Cursors from [FromState] awaiting the definition of the node they sit on, keyed by node ID.
	restore map[string][]*cursorData

	progressing bool
	updates     chan<- WorkflowUpdate // Valid while progressing
	pending     []WorkflowUpdate      // Emitted while not progressing; replayed by the next Progress
}

func New() Workflow {
	w := &workflow{
		nodes:   map[string]fsa.Node{},
		nodeIDs: map[fsa.Node]string{},
		funcs:   map[string]NodeFunc{},
		states:  map[string]NodeState{},
		touched: map[string]bool{},
		joins:   map[string]bool{},
		restore: map[string][]*cursorData{},
	}
	w.g = fsa.New(fsa.OnReplace(w.replaced))
	return Workflow{w}
}

// Report a cursor overwritten by the FSA. A cursor merged by a join already ended in CursorsJoined and
// left the machine; should a stale in-flight overwrite of it still arrive, it is not a second end.
func (w *workflow) replaced(old, by cursor, at fsa.Node) {
	w.m.Lock()
	consumed := old.consumed
	u := CursorReplaced{Old: old.Cursor(), New: by.Cursor(), Node: Node{w.nodeIDs[at]}}
	w.m.Unlock()
	if !consumed {
		w.emit(u)
	}
}

// FromState builds a Workflow whose cursors are those saved by [Workflow.State]. Each cursor is placed as
// soon as the node it sits on is defined; every [Workflow.Progress] reports cursors whose node is still
// undefined with [NodeUndefined].
func FromState(state json.RawMessage) (Workflow, error) {
	var s savedState
	dec := json.NewDecoder(bytes.NewReader(state))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Workflow{}, fmt.Errorf("invalid workflow state: %w", err)
	}
	w := New()
	w.nextCursor = s.NextCursor
	for _, c := range s.Cursors {
		entered, err := decodeProperties(c.Entered)
		if err != nil {
			return Workflow{}, fmt.Errorf("invalid workflow state: cursor %q: %w", c.ID, err)
		}
		data := &cursorData{id: c.ID, label: c.Label, entered: entered}
		if c.Outputs != nil {
			outputs, err := decodeProperties(*c.Outputs)
			if err != nil {
				return Workflow{}, fmt.Errorf("invalid workflow state: cursor %q: %w", c.ID, err)
			}
			data.outputs, data.ran = outputs, true
		}
		if c.Merged != nil {
			data.mergedJoin, data.mergedOn = c.Merged.Join, c.Merged.Node
		}
		w.restore[c.Node] = append(w.restore[c.Node], data)
	}
	return w, nil
}

type (
	// A NodeFunc runs when cursor c enters the node with inputs, the values it entered with. It returns the
	// values the cursor leaves the node with.
	NodeFunc = func(ctx context.Context, w Workflow, c Cursor, inputs property.Map) (property.Map, error)
	// An EdgeFunc decides whether cursor c, whose node produced inputs, may cross the edge. The returned
	// Overlay applies to the cursor's values if it crosses because of this decision; it is discarded
	// otherwise.
	EdgeFunc = func(ctx context.Context, w Workflow, c Cursor, inputs property.Map) (bool, Overlay, error)
)

// An Overlay is what an edge does to a cursor's values as the cursor crosses it: keys it deletes and
// values it sets.
type Overlay struct {
	Values  property.Map
	Deleted []string
}

func (o Overlay) MarshalJSON() ([]byte, error) {
	deleted := o.Deleted
	if deleted == nil {
		deleted = []string{}
	}
	return json.Marshal(struct {
		Values  any      `json:"values"`
		Deleted []string `json:"deleted"`
	}{propertyJSON(o.Values), deleted})
}

// Apply returns m with o's deletions and then its values applied.
func (o Overlay) Apply(m property.Map) property.Map {
	if len(o.Deleted) == 0 && o.Values.Len() == 0 {
		return m
	}
	out := m.AsMap()
	for _, k := range o.Deleted {
		delete(out, k)
	}
	maps.Insert(out, o.Values.All)
	return property.NewMap(out)
}

// over composes o on top of under: o's deletions and values win.
func (o Overlay) over(under Overlay) Overlay {
	values := under.Values.AsMap()
	for _, k := range o.Deleted {
		delete(values, k)
	}
	maps.Insert(values, o.Values.All)
	var deleted []string
	for _, k := range under.Deleted {
		if _, set := o.Values.GetOk(k); !set {
			deleted = append(deleted, k)
		}
	}
	for _, k := range o.Deleted {
		if !slices.Contains(deleted, k) {
			deleted = append(deleted, k)
		}
	}
	return Overlay{Values: property.NewMap(values), Deleted: deleted}
}

// Node identifies a node of a Workflow. It serializes as the node's ID.
type Node struct{ id string }

func (n Node) ID() string { return n.id }

func (n Node) MarshalJSON() ([]byte, error) {
	if n.id == "" {
		return []byte("null"), nil
	}
	return json.Marshal(n.id)
}

type Cursor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// NewNode defines a node. f runs each time a cursor enters the node: it receives the values the cursor
// entered with and its result becomes the values the cursor leaves with. Node IDs must be unique.
func (w Workflow) NewNode(id string, f NodeFunc) Node {
	contract.Assertf(id != "", "node IDs must not be empty")
	w.emit(NodeDefined{ID: id})
	w.register(id, f, w.g.NewNode(func(ctx context.Context, _ fsa.FSA[cursor], _ fsa.Edge, c cursor) error {
		w.m.Lock()
		inputs := c.overlay.Apply(c.value())
		c.overlay = Overlay{}
		clear(c.reached) // Entering a node starts a fresh visit
		w.m.Unlock()
		return w.run(ctx, id, f, c, inputs)
	}))
	return Node{id}
}

// Run node id's function for c with inputs, recording the result on the cursor.
func (w Workflow) run(ctx context.Context, id string, f NodeFunc, c cursor, inputs property.Map) error {
	w.m.Lock()
	w.touched[id] = true
	ident := c.Cursor()
	w.m.Unlock()
	w.emit(NodeStarted{ID: id, Cursor: ident, Inputs: inputs})
	outputs, err := f(ctx, w, ident, inputs)
	if err != nil {
		w.emit(NodeFailed{ID: id, Cursor: ident, Error: err})
		return err
	}
	w.m.Lock()
	// A node function that returns nil has its cursor committed to the node, so the values may be
	// recorded at once.
	c.entered, c.outputs, c.ran = inputs, outputs, true
	w.states[id] = NodeState{LastRun: time.Now(), Inputs: inputs, Outputs: outputs}
	w.m.Unlock()
	w.emit(NodeSucceeded{ID: id, Cursor: ident, Outputs: outputs})
	return nil
}

// AddCursor places a cursor with the given inputs on n. The node's function is not run. Placing is an
// arrival: a cursor already on n is overwritten once it settles there without moving away.
func (w Workflow) AddCursor(n Node, label string, inputs property.Map) {
	fn := w.fsaNode(n)
	w.m.Lock()
	c := &cursorData{id: w.newCursorIDLocked(), label: label, entered: inputs}
	w.m.Unlock()
	w.emit(CursorAdded{Node: n, Cursor: c.Cursor(), Inputs: inputs})
	w.m.Lock()
	c.ref = w.g.NewCursor(cursor{c}, fn) // Placed and referenced under one lock: no one sees it unreferenced
	w.m.Unlock()
}

// NewEdge defines an edge from one node to another, taken when edge returns true.
//
// A node's outgoing edges are asked in definition order and the first to pass is taken.
func (w Workflow) NewEdge(name string, from, to Node, edge EdgeFunc) {
	ident := EdgeIdentity{Name: name, From: from, To: to}
	w.addEdge(EdgeDefined{EdgeIdentity: ident}, from, func(ctx context.Context, c cursor) (bool, Overlay, error) {
		return w.eval(ctx, ident, edge, c)
	})
}

// NewOrEdge defines an edge taken when any of conds returns true. Conditions are asked one at a time in
// the order of their names and the first true short-circuits; only its Overlay applies.
func (w Workflow) NewOrEdge(name string, from, to Node, conds map[string]EdgeFunc) {
	w.composite(name, from, to, conds, false)
}

// NewAndEdge defines an edge taken when all of conds return true. Conditions are asked together; their
// Overlays compose, the first in the order of their names winning a key several set or delete.
func (w Workflow) NewAndEdge(name string, from, to Node, conds map[string]EdgeFunc) {
	w.composite(name, from, to, conds, true)
}

func (w Workflow) composite(name string, from, to Node, conds map[string]EdgeFunc, all bool) {
	contract.Assertf(len(conds) > 0, "edge %q: at least one condition is required", name)
	ident := EdgeIdentity{Name: name, From: from, To: to}
	def := EdgeDefined{EdgeIdentity: ident}
	names := slices.Sorted(maps.Keys(conds))
	children := make([]EdgeIdentity, len(names))
	for i, cond := range names {
		children[i] = EdgeIdentity{Name: name, Condition: cond, From: from, To: to}
		if all {
			def.AndEdges = append(def.AndEdges, EdgeDefined{EdgeIdentity: children[i]})
		} else {
			def.OrEdges = append(def.OrEdges, EdgeDefined{EdgeIdentity: children[i]})
		}
	}
	w.addEdge(def, from, func(ctx context.Context, c cursor) (bool, Overlay, error) {
		w.emit(EdgeStarted{EdgeIdentity: ident, Cursor: c.Cursor(), Inputs: c.get(w.workflow)})
		var pass bool
		var outputs Overlay
		var err error
		if all {
			pass, outputs, err = w.evalAll(ctx, children, names, conds, c)
		} else {
			pass, outputs, err = w.evalAny(ctx, children, names, conds, c)
		}
		if err != nil {
			w.emit(EdgeFailed{EdgeIdentity: ident, Cursor: c.Cursor(), Error: err})
			return false, Overlay{}, err
		}
		w.emit(EdgeFinished{EdgeIdentity: ident, Cursor: c.Cursor(), Pass: pass, Overlay: outputs})
		return pass, outputs, nil
	})
}

// Ask every condition at once. Events are reported in name order once all have answered, so that the
// stream is deterministic.
func (w Workflow) evalAll(
	ctx context.Context, children []EdgeIdentity, names []string, conds map[string]EdgeFunc, c cursor,
) (bool, Overlay, error) {
	ident, inputs := c.Cursor(), c.get(w.workflow)
	for _, child := range children {
		w.emit(EdgeStarted{EdgeIdentity: child, Cursor: ident, Inputs: inputs})
	}
	type answer struct {
		pass    bool
		outputs Overlay
		err     error
	}
	answers := make([]answer, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			if err := ctx.Err(); err != nil {
				answers[i].err = err
				return
			}
			pass, outputs, err := conds[name](ctx, w, ident, inputs)
			answers[i] = answer{pass, outputs, err}
		})
	}
	wg.Wait()
	pass := true
	var errs []error
	for i, a := range answers {
		if a.err != nil {
			w.emit(EdgeFailed{EdgeIdentity: children[i], Cursor: ident, Error: a.err})
			errs = append(errs, a.err)
			continue
		}
		w.emit(EdgeFinished{EdgeIdentity: children[i], Cursor: ident, Pass: a.pass, Overlay: a.outputs})
		pass = pass && a.pass
	}
	if len(errs) > 0 {
		return false, Overlay{}, errors.Join(errs...)
	}
	var outputs Overlay
	for i := len(answers) - 1; i >= 0; i-- {
		outputs = answers[i].outputs.over(outputs) // Earlier names win
	}
	return pass, outputs, nil
}

// Ask conditions one at a time until one passes.
func (w Workflow) evalAny(
	ctx context.Context, children []EdgeIdentity, names []string, conds map[string]EdgeFunc, c cursor,
) (bool, Overlay, error) {
	for i, name := range names {
		if err := ctx.Err(); err != nil {
			return false, Overlay{}, err // The run was aborted: ask nothing more
		}
		pass, outputs, err := w.eval(ctx, children[i], conds[name], c)
		if err != nil || pass {
			return pass, outputs, err
		}
	}
	return false, Overlay{}, nil
}

type JoinEdgeArg struct {
	From Node
	Edge EdgeFunc
}

// A MergeFunc decides a join once every branch's Edge has passed: true merges the candidates into one
// cursor described by MergedCursor; false rejects the merge, leaving every candidate where it is to be
// asked again later; an error fails the Progress.
type MergeFunc = func(ctx context.Context, candidates []MergeCandidate) (bool, MergedCursor, error)

// MergeCandidate is a cursor waiting on a join branch, in the order of the join's From nodes. Inputs are
// the values its node produced, with its branch Edge's Overlay applied.
type MergeCandidate struct {
	From   Node
	Cursor Cursor
	Inputs property.Map
}

type MergedCursor struct {
	Label  string
	Inputs property.Map
}

// NewJoinEdge defines an edge that merges one cursor from each of from into a single cursor at to. The
// join is taken only when every From node holds a cursor and, evaluated together at that moment, every
// branch's Edge returns true for its cursor and merge accepts them; otherwise every cursor stays where
// it is. The join's own [EdgeFinished] reports merge's decision.
//
// Each From node may have other outgoing edges, asked in definition order like any node's: a cursor is
// only merged once the edges defined before the join have failed for it. Branch Edges and merge are
// asked together on every attempt at the join, so unlike other edges they may be asked more than once
// per visit. A join is decided exactly once: should the merged cursor's move to to be interrupted, it
// is completed on a later Progress rather than re-decided. Join names must be unique within a Workflow
// and From nodes distinct.
func (w Workflow) NewJoinEdge(name string, from []JoinEdgeArg, to Node, merge MergeFunc) {
	contract.Assertf(len(from) > 0, "edge %q: at least one branch is required", name)
	contract.Assertf(merge != nil, "edge %q: a merge function is required", name)
	w.m.Lock()
	contract.Assertf(!w.joins[name], "join %q is already defined", name)
	w.joins[name] = true
	w.m.Unlock()
	def := EdgeDefined{EdgeIdentity: EdgeIdentity{Name: name, To: to}}
	j := &join{
		ident: def.EdgeIdentity, from: from, merge: merge,
		branches: make([]EdgeIdentity, len(from)), nodes: make([]fsa.Node, len(from)),
	}
	for i, arg := range from {
		contract.Assertf(arg.From != to, "join %q: a join cannot target one of its own From nodes", name)
		contract.Assertf(!slices.ContainsFunc(from[:i], func(o JoinEdgeArg) bool { return o.From == arg.From }),
			"join %q: From node %q is listed twice", name, arg.From.id)
		j.branches[i] = EdgeIdentity{Name: name, From: arg.From, To: to}
		j.nodes[i] = w.fsaNode(arg.From)
		def.JoinEdges = append(def.JoinEdges, EdgeDefined{EdgeIdentity: j.branches[i]})
	}
	target := w.fsaNode(to)
	w.emit(def)
	for i, arg := range from {
		w.g.NewEdge(w.guard(arg.From, name, w.joinCond(j, i)), j.nodes[i], target)
	}
}

type join struct {
	ident    EdgeIdentity // Name and To; From is unset
	from     []JoinEdgeArg
	merge    MergeFunc
	branches []EdgeIdentity
	nodes    []fsa.Node // from[i].From
}

// The condition asked of a cursor on from[k]: pass for the cursor that finds every branch occupied and
// every branch edge passing, merging the others into it.
func (w Workflow) joinCond(j *join, k int) func(context.Context, cursor) (bool, Overlay, error) {
	return func(ctx context.Context, me cursor) (bool, Overlay, error) {
		none := Overlay{}
		w.m.Lock()
		if me.mergedJoin == j.ident.Name && me.mergedOn == j.from[k].From.id {
			w.m.Unlock()
			return true, none, nil // Already decided; this is the interrupted move being completed
		}
		if me.reached == nil {
			me.reached = map[*join]bool{}
		}
		me.reached[j] = true // Every edge before this join has failed for me this visit
		present, ok := w.participantsLocked(j)
		w.m.Unlock()
		if !ok || present[k].cursorData != me.cursorData {
			return false, none, nil
		}
		candidates := make([]MergeCandidate, len(present))
		for i, p := range present {
			if err := ctx.Err(); err != nil {
				return false, none, err // The run was aborted: ask nothing more
			}
			pass, outputs, err := w.evalValues(ctx, j.branches[i], j.from[i].Edge, p.ident, p.value)
			if err != nil || !pass {
				return false, none, err
			}
			candidates[i] = MergeCandidate{From: j.from[i].From, Cursor: p.ident, Inputs: outputs.Apply(p.value)}
		}
		if err := ctx.Err(); err != nil {
			return false, none, err
		}
		accept, merged, err := j.merge(ctx, candidates)
		if err != nil {
			w.emit(EdgeFailed{EdgeIdentity: j.ident, Cursor: me.Cursor(), Error: err})
			return false, none, err
		}
		w.emit(EdgeFinished{
			EdgeIdentity: j.ident, Cursor: me.Cursor(), Pass: accept, Overlay: Overlay{Values: merged.Inputs},
		})
		if !accept {
			return false, none, nil
		}

		w.m.Lock()
		// The branch edges ran without the lock: decide only if nothing changed underneath them.
		again, ok := w.participantsLocked(j)
		if !ok || me.consumed {
			w.m.Unlock()
			return false, none, nil
		}
		for i := range present {
			if again[i].cursorData != present[i].cursorData || !again[i].value.Equals(present[i].value) {
				w.m.Unlock()
				return false, none, nil
			}
		}
		// Merging is the others' move: they leave the machine now, atomically, so that nothing observes
		// them as arrivals afterwards. If one is already gone the attempt is void.
		var others []fsa.CursorRef
		for _, p := range present {
			if p.cursorData != me.cursorData {
				others = append(others, p.ref)
			}
		}
		if !w.g.Remove(others...) {
			w.m.Unlock()
			return false, none, nil
		}
		joined := CursorsJoined{New: Cursor{ID: w.newCursorIDLocked(), Label: merged.Label}}
		for _, p := range present {
			joined.Old = append(joined.Old, p.ident)
			p.consumed = p.cursorData != me.cursorData // Its own in-flight conditions must not act
		}
		me.id, me.label = joined.New.ID, joined.New.Label
		me.entered, me.outputs, me.ran, me.overlay = merged.Inputs, property.Map{}, false, Overlay{}
		me.mergedJoin, me.mergedOn = j.ident.Name, j.from[k].From.id
		me.leaving = true // Decided: no other join may take me from here on
		w.m.Unlock()
		w.emit(joined)
		return true, none, nil
	}
}

// A join participant: a live cursor on one of the join's From nodes, with its identity and value as
// seen under the lock.
type participant struct {
	cursor
	ident Cursor
	value property.Map
}

// The participant on each of j's From nodes, if every node has one. A cursor participates once it has
// reached the join this visit — every edge defined before it has failed — and is not moving away.
func (w *workflow) participantsLocked(j *join) ([]participant, bool) {
	present := make([]participant, len(j.nodes))
	for c, n := range w.g.Cursors {
		if c.consumed || c.leaving || c.mergedOn != "" || !c.reached[j] {
			continue // Gone, going, committed to a decided join, or not yet at this join
		}
		if i := slices.Index(j.nodes, n); i >= 0 && present[i].cursorData == nil {
			present[i] = participant{c, c.Cursor(), c.value()}
		}
	}
	for _, p := range present {
		if p.cursorData == nil {
			return nil, false
		}
	}
	return present, true
}

func (w Workflow) addEdge(def EdgeDefined, from Node, cond func(context.Context, cursor) (bool, Overlay, error)) {
	fromNode, to := w.fsaNode(def.From), w.fsaNode(def.To)
	w.emit(def)
	w.g.NewEdge(w.guard(from, "", cond), fromNode, to)
}

// guard wraps a condition asked of cursors on from with the bookkeeping every edge shares: a cursor
// merged away by a join never moves, a cursor whose join was decided here takes nothing but that join
// (named by joinName for the join's own edges), and a pass is recorded atomically with the check so
// that a join cannot merge a cursor that is leaving.
func (w *workflow) guard(
	from Node, joinName string, cond func(context.Context, cursor) (bool, Overlay, error),
) fsa.ConditionFunc[cursor] {
	return func(ctx context.Context, _ fsa.FSA[cursor], c cursor) (fsa.ConditionResult, error) {
		w.m.Lock()
		c.leaving = false     // Being asked here means the cursor is here
		c.overlay = Overlay{} // And that no earlier pass completed
		if c.mergedOn != "" && c.mergedOn != from.id {
			c.mergedJoin, c.mergedOn = "", "" // Being asked elsewhere means the merged cursor's move completed
		}
		blocked := c.consumed || (c.mergedOn == from.id && c.mergedJoin != joinName)
		w.m.Unlock()
		if blocked {
			return fsa.ConditionFail, nil
		}
		pass, outputs, err := cond(ctx, c)
		if err != nil {
			return fsa.ConditionUnknown, err
		}
		if !pass {
			return fsa.ConditionFail, nil
		}
		w.m.Lock()
		defer w.m.Unlock()
		if c.consumed {
			return fsa.ConditionFail, nil // Merged away while deciding; the merged cursor carries on
		}
		c.leaving, c.overlay = true, outputs
		return fsa.ConditionPass, nil
	}
}

// Ask edge of c, reporting the outcome as events.
func (w Workflow) eval(ctx context.Context, ident EdgeIdentity, edge EdgeFunc, c cursor) (bool, Overlay, error) {
	return w.evalValues(ctx, ident, edge, c.Cursor(), c.get(w.workflow))
}

func (w Workflow) evalValues(
	ctx context.Context, ident EdgeIdentity, edge EdgeFunc, c Cursor, inputs property.Map,
) (bool, Overlay, error) {
	w.emit(EdgeStarted{EdgeIdentity: ident, Cursor: c, Inputs: inputs})
	pass, outputs, err := edge(ctx, w, c, inputs)
	if err != nil {
		w.emit(EdgeFailed{EdgeIdentity: ident, Cursor: c, Error: err})
		return false, Overlay{}, err
	}
	w.emit(EdgeFinished{EdgeIdentity: ident, Cursor: c, Pass: pass, Overlay: outputs})
	return pass, outputs, nil
}

// NodeByID looks up a defined node by its ID.
func (w Workflow) NodeByID(id string) (Node, bool) {
	w.m.Lock()
	defer w.m.Unlock()
	_, ok := w.nodes[id]
	return Node{id}, ok
}

// GetState reports the last run of node; the zero value if it has not run.
func (w Workflow) GetState(node Node) NodeState {
	w.m.Lock()
	defer w.m.Unlock()
	return w.states[node.id]
}

// Cursors calls yield for every placed cursor, in arrival order, with the node it sits on. Cursors
// restored onto nodes not yet defined are not placed and are reported by [NodeUndefined] instead.
func (w Workflow) Cursors(yield func(Cursor, Node) bool) {
	w.m.Lock()
	defer w.m.Unlock()
	for c, n := range w.g.Cursors {
		if !c.consumed && !yield(c.Cursor(), Node{w.nodeIDs[n]}) {
			return
		}
	}
}

// Progress iterates the workflow until no cursor can move, sending events to updates (which may be nil).
// Events emitted before Progress (definitions, added cursors) are sent first. Once no cursor can move,
// Progress reports each node holding restored cursors that is still undefined ([NodeUndefined]) and each
// defined node whose function did not run ([NodeUntouched]).
func (w Workflow) Progress(
	ctx context.Context, runner fsa.Runner, updates chan<- WorkflowUpdate,
) error {
	w.m.Lock()
	contract.Assertf(!w.progressing, "we can only progress one run at a time")
	w.progressing, w.updates = true, updates
	clear(w.touched)
	for c := range w.g.Cursors {
		// Every Progress starts a fresh visit: a move interrupted by a failed run is retried this run.
		c.leaving = false
		clear(c.reached)
	}
	pending := w.pending
	w.pending = nil
	w.m.Unlock()
	defer func() {
		w.m.Lock()
		w.progressing, w.updates = false, nil
		w.m.Unlock()
	}()

	if updates != nil {
		for _, u := range pending {
			updates <- u
		}
	}
	err := w.g.Progress(ctx, runner)

	w.m.Lock()
	var summary []WorkflowUpdate
	for _, id := range slices.Sorted(maps.Keys(w.restore)) {
		u := NodeUndefined{ID: id}
		for _, c := range w.restore[id] {
			u.Cursors = append(u.Cursors, c.Cursor())
		}
		summary = append(summary, u)
	}
	for _, id := range w.defined {
		if !w.touched[id] {
			summary = append(summary, NodeUntouched{ID: id})
		}
	}
	w.m.Unlock()
	for _, u := range summary {
		w.emit(u)
	}
	return err
}

// Reconcile re-runs the node function for every cursor sitting on a node whose function did not run
// during the preceding [Workflow.Progress], from the values the cursor entered the node with, and
// replaces the values the cursor leaves with. Nodes are reconciled through runner, which may run them
// concurrently; cursors sharing a node are reconciled one after the other. Every node is attempted and
// their errors are joined. Events go to updates (which may be nil).
func (w Workflow) Reconcile(ctx context.Context, runner fsa.Runner, updates chan<- WorkflowUpdate) error {
	w.m.Lock()
	contract.Assertf(!w.progressing, "we can only progress one run at a time")
	w.progressing, w.updates = true, updates
	byNode := map[string][]cursor{}
	var order []string
	for c, n := range w.g.Cursors {
		id := w.nodeIDs[n]
		if c.consumed || w.touched[id] {
			continue
		}
		if _, seen := byNode[id]; !seen {
			order = append(order, id)
		}
		byNode[id] = append(byNode[id], c)
	}
	w.m.Unlock()
	defer func() {
		w.m.Lock()
		w.progressing, w.updates = false, nil
		w.m.Unlock()
	}()

	var wg sync.WaitGroup
	errs := make([]error, len(order))
	for i, id := range order {
		wg.Add(1)
		err := runner(ctx, func(ctx context.Context) {
			defer wg.Done()
			var nodeErrs []error
			for _, c := range byNode[id] {
				w.m.Lock()
				entered := c.entered
				w.m.Unlock()
				if err := w.run(ctx, id, w.funcs[id], c, entered); err != nil {
					nodeErrs = append(nodeErrs, fmt.Errorf("node %q: %w", id, err))
				}
			}
			errs[i] = errors.Join(nodeErrs...)
		})
		if err != nil {
			wg.Done()
			errs[i] = err
		}
	}
	wg.Wait()
	return errors.Join(errs...)
}

// State serializes the workflow's cursors so that [FromState] can resume them once the same nodes and
// edges are defined. Cursors sharing a node are listed in arrival order, which [FromState] preserves, so
// a cursor that arrived beside an occupant still to settle keeps precedence over it. It must not be
// called while Progress is running.
func (w Workflow) State() json.RawMessage {
	w.m.Lock()
	defer w.m.Unlock()
	contract.Assertf(!w.progressing, "State cannot be taken while progressing")
	s := savedState{NextCursor: w.nextCursor}
	save := func(c *cursorData, node string) {
		sc := savedCursor{ID: c.id, Label: c.label, Node: node, Entered: encodeProperties(c.entered)}
		if c.ran {
			outputs := encodeProperties(c.outputs)
			sc.Outputs = &outputs
		}
		if c.mergedOn != "" {
			sc.Merged = &savedMerge{Join: c.mergedJoin, Node: c.mergedOn}
		}
		s.Cursors = append(s.Cursors, sc)
	}
	for c, n := range w.g.Cursors {
		if !c.consumed {
			save(c.cursorData, w.nodeIDs[n])
		}
	}
	for _, node := range slices.Sorted(maps.Keys(w.restore)) {
		for _, c := range w.restore[node] {
			save(c, node)
		}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	contract.AssertNoErrorf(err, "state is always serializable")
	return b
}

type savedState struct {
	NextCursor int           `json:"nextCursor"`
	Cursors    []savedCursor `json:"cursors"`
}

type savedCursor struct {
	ID      string          `json:"id"`
	Label   string          `json:"label"`
	Node    string          `json:"node"`
	Entered map[string]any  `json:"entered"`
	Outputs *map[string]any `json:"outputs,omitempty"`
	Merged  *savedMerge     `json:"merged,omitempty"`
}

type savedMerge struct {
	Join string `json:"join"`
	Node string `json:"node"`
}

// Property maps are stored in the format of a stack's deployment, which keeps secrets, unknowns,
// assets, resource references and non-finite numbers. Secrets are stored in the clear.
func encodeProperties(m property.Map) map[string]any {
	v, err := stack.SerializeProperties(context.Background(),
		resource.ToResourcePropertyMap(m), config.NopEncrypter, false /* showSecrets */)
	contract.AssertNoErrorf(err, "property maps are always serializable")
	if v == nil {
		v = map[string]any{}
	}
	return v
}

func decodeProperties(v map[string]any) (property.Map, error) {
	m, err := stack.DeserializeProperties(v, config.NopDecrypter)
	if err != nil {
		return property.Map{}, err
	}
	return resource.FromResourcePropertyMap(m), nil
}

type NodeState struct {
	LastRun time.Time
	Inputs  property.Map
	Outputs property.Map
}

func (w *workflow) emit(u WorkflowUpdate) {
	w.m.Lock()
	if !w.progressing {
		w.pending = append(w.pending, u)
		w.m.Unlock()
		return
	}
	ch := w.updates
	w.m.Unlock()
	if ch != nil {
		ch <- u
	}
}

// Record n under id, placing any restored cursors waiting for it.
func (w *workflow) register(id string, f NodeFunc, n fsa.Node) {
	w.m.Lock()
	_, dup := w.nodes[id]
	contract.Assertf(!dup, "node %q is already defined", id)
	w.nodes[id], w.nodeIDs[n], w.funcs[id] = n, id, f
	w.defined = append(w.defined, id)
	restored := w.restore[id]
	delete(w.restore, id)
	for _, c := range restored {
		c.ref = w.g.NewCursor(cursor{c}, n) // In saved order, which is arrival order
	}
	w.m.Unlock()
}

func (w *workflow) fsaNode(n Node) fsa.Node {
	w.m.Lock()
	defer w.m.Unlock()
	fn, ok := w.nodes[n.id]
	contract.Assertf(ok, "node %q is not defined in this workflow", n.id)
	return fn
}

func (w *workflow) newCursorIDLocked() string {
	w.nextCursor++
	return fmt.Sprintf("c%d", w.nextCursor)
}

// The FSA cursor. It is a pointer so that node functions, which receive the cursor by value, can update
// it. Fields are guarded by workflow.m.
type cursor struct{ *cursorData }

type cursorData struct {
	id, label string
	ref       fsa.CursorRef // Valid once placed
	// The values the cursor entered its node with, and what the node's function made of them, if it has
	// run for this visit.
	entered    property.Map
	outputs    property.Map
	ran        bool
	overlay    Overlay        // Of the edge the cursor is crossing; applied as it enters the next node
	consumed   bool           // Merged into another cursor by a join and removed; inert if still asked
	leaving    bool           // A condition passed; the cursor is moving away and must not be merged
	reached    map[*join]bool // Joins asked of this cursor during its current visit
	mergedJoin string         // With mergedOn: a join decided for this cursor while on that node, until it moves on
	mergedOn   string
}

// The values the cursor leaves its node with.
func (c *cursorData) value() property.Map {
	if c.ran {
		return c.outputs
	}
	return c.entered
}

func (c cursor) get(w *workflow) property.Map {
	w.m.Lock()
	defer w.m.Unlock()
	return c.value()
}

func (c *cursorData) Cursor() Cursor { return Cursor{ID: c.id, Label: c.label} }
