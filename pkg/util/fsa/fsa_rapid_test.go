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

package fsa_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
)

// A generated machine, described as plain data so a single spec can be built and run repeatedly.
//
// Conditions are scripted per (edge, cursor): the nth ask returns the nth script element, and an
// exhausted (or absent) script fails forever. Scripts are finite and pre-drawn, so total passes — and
// with them moves, visits, and asks — are bounded, guaranteeing termination even on cyclic graphs and
// in the presence of mid-run mutations.
type machineSpec struct {
	numNodes  int
	edges     []edgeSpec
	initial   []int // initial cursor value -> starting node index
	total     int   // total cursors ever created (initial + mutation-added)
	mutations []mutationSpec
}

type edgeSpec struct {
	from, to int
	scripts  [][]bool // cursor value -> results script (true = pass); exhausted -> fail
}

// A mid-run mutation, fired from inside a callback once the global event counter (asks + entries)
// reaches afterEvents. edge == nil means "add a cursor at cursorAt".
type mutationSpec struct {
	afterEvents int
	edge        *edgeSpec
	cursorAt    int
}

func genEdge(t *rapid.T, label string, numNodes, totalCursors int) edgeSpec {
	es := edgeSpec{
		from:    rapid.IntRange(0, numNodes-1).Draw(t, label+"From"),
		to:      rapid.IntRange(0, numNodes-1).Draw(t, label+"To"),
		scripts: make([][]bool, totalCursors),
	}
	for c := range totalCursors {
		es.scripts[c] = rapid.SliceOfN(rapid.Bool(), 0, 3).Draw(t, fmt.Sprintf("%sScript%d", label, c))
	}
	return es
}

func genSpec(t *rapid.T) machineSpec {
	shape := rapid.SampledFrom([]string{"uniform", "ring", "sparse"}).Draw(t, "shape")

	var spec machineSpec
	numInitial := rapid.IntRange(1, 4).Draw(t, "numCursors")
	numMut := rapid.IntRange(0, 3).Draw(t, "numMutations")
	addsCursor := make([]bool, numMut)
	for i := range numMut {
		addsCursor[i] = rapid.Bool().Draw(t, fmt.Sprintf("mut%dAddsCursor", i))
	}
	spec.total = numInitial
	for _, b := range addsCursor {
		if b {
			spec.total++
		}
	}

	switch shape {
	case "ring":
		// A full cycle where each ring edge passes once, for the first cursor placed at its source.
		// With every cursor moving at once this drives the mutual-deferral (stall) path; co-located
		// extra cursors park and get overwritten by the rotation.
		spec.numNodes = rapid.IntRange(2, 6).Draw(t, "numNodes")
		passOwner := map[int]int{}
		for c := range numInitial {
			at := c % spec.numNodes
			spec.initial = append(spec.initial, at)
			if _, taken := passOwner[at]; !taken {
				passOwner[at] = c
			}
		}
		for i := range spec.numNodes {
			es := edgeSpec{from: i, to: (i + 1) % spec.numNodes, scripts: make([][]bool, spec.total)}
			if owner, ok := passOwner[i]; ok {
				es.scripts[owner] = []bool{true}
			}
			spec.edges = append(spec.edges, es)
		}
		for e := range rapid.IntRange(0, 2).Draw(t, "numChords") {
			spec.edges = append(spec.edges, genEdge(t, fmt.Sprintf("chord%d", e), spec.numNodes, spec.total))
		}
	case "sparse":
		// Few edges over many nodes: plenty of edge-less resting spots and overwrite targets.
		spec.numNodes = rapid.IntRange(4, 8).Draw(t, "numNodes")
		for e := range rapid.IntRange(0, 3).Draw(t, "numEdges") {
			spec.edges = append(spec.edges, genEdge(t, fmt.Sprintf("edge%d", e), spec.numNodes, spec.total))
		}
		for c := range numInitial {
			spec.initial = append(spec.initial,
				rapid.IntRange(0, spec.numNodes-1).Draw(t, fmt.Sprintf("cursor%dAt", c)))
		}
	default: // uniform
		spec.numNodes = rapid.IntRange(1, 6).Draw(t, "numNodes")
		for e := range rapid.IntRange(0, 10).Draw(t, "numEdges") {
			spec.edges = append(spec.edges, genEdge(t, fmt.Sprintf("edge%d", e), spec.numNodes, spec.total))
		}
		for c := range numInitial {
			spec.initial = append(spec.initial,
				rapid.IntRange(0, spec.numNodes-1).Draw(t, fmt.Sprintf("cursor%dAt", c)))
		}
	}

	for i := range numMut {
		mut := mutationSpec{afterEvents: rapid.IntRange(1, 12).Draw(t, fmt.Sprintf("mut%dAfter", i))}
		if addsCursor[i] {
			mut.cursorAt = rapid.IntRange(0, spec.numNodes-1).Draw(t, fmt.Sprintf("mut%dCursorAt", i))
		} else {
			e := genEdge(t, fmt.Sprintf("mut%dEdge", i), spec.numNodes, spec.total)
			mut.edge = &e
		}
		spec.mutations = append(spec.mutations, mut)
	}
	slices.SortStableFunc(spec.mutations, func(a, b mutationSpec) int { return a.afterEvents - b.afterEvents })
	return spec
}

type runResult struct {
	log        []string // every condition ask and node entry, in order
	violations []string
	err        error

	final   map[int]int   // cursor value -> node index at quiescence
	parked  map[int]int   // cursor value -> node index, as reported by Parked()
	outDeg  []int         // node index -> out-degree at quiescence
	passes  map[int]int   // cursor value -> conditions passed
	entries map[int]int   // cursor value -> nodes entered
	lastAt  map[int]int   // cursor value -> last known node index
	entered map[int][]int // node index -> cursor values that entered it, in order
	created int
}

// Build the spec into a machine and run one Progress over it, recording every callback. Visits are
// derived from entry calls: a cursor's visit count bumps each time it enters a node (initial placement
// is visit 0). Draw-free, so a spec can be re-run for determinism checks. Instrumentation is guarded
// by the machine's own serialization under SyncRunner and by mu under concurrent runners.
func runSpec(spec machineSpec, runner fsa.Runner) *runResult {
	var mu sync.Mutex
	res := &runResult{
		final:   map[int]int{},
		parked:  map[int]int{},
		outDeg:  make([]int, spec.numNodes),
		passes:  map[int]int{},
		entries: map[int]int{},
		lastAt:  map[int]int{},
		entered: map[int][]int{},
	}
	machine := fsa.New[int]()

	visits := make([]int, spec.total)
	nodes := make([]fsa.Node, spec.numNodes)
	nodeIndex := map[fsa.Node]int{}
	events := 0
	pending := slices.Clone(spec.mutations)

	var fireDue func()

	for i := range nodes {
		nodes[i] = machine.NewNode(func(_ context.Context, _ fsa.FSA[int], _ fsa.Edge, v int) error {
			mu.Lock()
			visits[v]++
			res.entries[v]++
			res.lastAt[v] = i
			res.entered[i] = append(res.entered[i], v)
			res.log = append(res.log, fmt.Sprintf("enter n%d c%d", i, v))
			events++
			mu.Unlock()
			fireDue()
			return nil
		})
		nodeIndex[nodes[i]] = i
	}

	asks := map[[2]int]int{}  // (edge id, cursor value) -> times asked
	seen := map[[3]int]bool{} // (edge id, cursor value, visit)
	nextEdge := 0
	addEdge := func(es edgeSpec) {
		mu.Lock()
		eid := nextEdge
		nextEdge++
		res.outDeg[es.from]++
		mu.Unlock()
		machine.NewEdge(func(_ context.Context, _ fsa.FSA[int], v int) (fsa.ConditionResult, error) {
			mu.Lock()
			key := [3]int{eid, v, visits[v]}
			if seen[key] {
				res.violations = append(res.violations,
					fmt.Sprintf("edge %d asked twice for cursor %d during visit %d", eid, v, visits[v]))
			}
			seen[key] = true
			n := asks[[2]int{eid, v}]
			asks[[2]int{eid, v}]++
			events++
			pass := n < len(es.scripts[v]) && es.scripts[v][n]
			res.log = append(res.log, fmt.Sprintf("ask e%d c%d visit%d -> %t", eid, v, visits[v], pass))
			if pass {
				res.passes[v]++
			}
			mu.Unlock()
			fireDue()
			if pass {
				return fsa.ConditionPass, nil
			}
			return fsa.ConditionFail, nil
		}, nodes[es.from], nodes[es.to])
	}
	addCursor := func(at int) {
		mu.Lock()
		v := res.created
		res.created++
		res.lastAt[v] = at
		mu.Unlock()
		machine.NewCursor(v, nodes[at])
	}

	fireDue = func() {
		for {
			mu.Lock()
			if len(pending) == 0 || pending[0].afterEvents > events {
				mu.Unlock()
				return
			}
			mut := pending[0]
			pending = pending[1:]
			mu.Unlock()
			// Apply outside mu: addEdge/addCursor take the machine's own lock, and the machine may
			// re-enter our callbacks (which take mu) on other goroutines.
			if mut.edge != nil {
				addEdge(*mut.edge)
			} else {
				addCursor(mut.cursorAt)
			}
		}
	}

	for _, es := range spec.edges {
		addEdge(es)
	}
	for _, at := range spec.initial {
		addCursor(at)
	}

	res.err = machine.Progress(context.Background(), runner)

	for v, n := range maps.Collect(machine.Cursors) {
		res.final[v] = nodeIndex[n]
	}
	for v, n := range maps.Collect(machine.Parked) {
		res.parked[v] = nodeIndex[n]
	}
	return res
}

func checkOutcome(t *rapid.T, res *runResult) {
	if res.err != nil {
		// A race between two arrivals is a legal outcome of an arbitrary graph; anything else is not.
		// A failing run aborts mid-flight, so the quiescence assertions below do not apply.
		require.ErrorContains(t, res.err, "both moving")
		require.Empty(t, res.violations)
		return
	}
	require.Empty(t, res.violations)

	// Parked is exactly the cursors on nodes with outgoing edges; the rest are resting.
	wantParked := map[int]int{}
	for v, n := range res.final {
		if res.outDeg[n] > 0 {
			wantParked[v] = n
		}
	}
	require.Equal(t, wantParked, res.parked)

	// Every pass grants exactly one entry (eager entry, inevitable commit).
	require.Equal(t, res.passes, res.entries)

	// Cursor conservation: survivors were created, and every vanished cursor was overwritten — some
	// other cursor entered the node it was last seen on.
	for v := range res.final {
		require.Less(t, v, res.created)
	}
	for v := range res.created {
		if _, alive := res.final[v]; alive {
			continue
		}
		last := res.lastAt[v]
		overwritten := slices.ContainsFunc(res.entered[last], func(o int) bool { return o != v })
		require.True(t, overwritten,
			"cursor %d vanished from node %d, which no other cursor ever entered", v, last)
	}
}

// Within one visit to a node, each of the node's conditions is asked at most once per cursor. Only
// re-entering the node (a move) re-samples them. Also checks the quiescence invariants: Parked
// reporting, one entry per pass, and cursor conservation.
func TestRapidConditionOncePerVisit(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		checkOutcome(t, runSpec(genSpec(t), fsa.SyncRunner))
	})
}

// The same property and quiescence invariants hold when the runner spreads every callback across
// goroutines. Outcomes are timing-dependent (races legal), but never inconsistent.
func TestRapidConcurrentRunner(t *testing.T) {
	t.Parallel()

	goRunner := fsa.Runner(func(ctx context.Context, f func(context.Context)) error {
		go f(ctx)
		return nil
	})
	rapid.Check(t, func(t *rapid.T) {
		checkOutcome(t, runSpec(genSpec(t), goRunner))
	})
}

// Under SyncRunner, Progress is fully deterministic: two runs of the same machine produce identical
// callback sequences and identical outcomes.
func TestRapidDeterministic(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		spec := genSpec(t)
		first := runSpec(spec, fsa.SyncRunner)
		second := runSpec(spec, fsa.SyncRunner)
		checkOutcome(t, first)

		require.Equal(t, first.log, second.log)
		require.Equal(t, fmt.Sprint(first.err), fmt.Sprint(second.err))
		require.Equal(t, first.final, second.final)
	})
}
