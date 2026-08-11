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
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
)

// A generated machine, described as plain data so a single spec can be built and run repeatedly.
// Conditions are scripted per (edge, cursor) with a pass budget of at most one, which bounds total
// moves and so guarantees termination.
type machineSpec struct {
	numNodes int
	edges    [][2]int        // (from, to) node indices
	passes   map[[2]int]bool // (edge index, cursor value): passes on first ask
	cursors  []int           // cursor value -> starting node index
}

func genSpec(t *rapid.T) machineSpec {
	numNodes := rapid.IntRange(1, 6).Draw(t, "numNodes")
	numEdges := rapid.IntRange(0, 10).Draw(t, "numEdges")
	numCursors := rapid.IntRange(1, 4).Draw(t, "numCursors")

	spec := machineSpec{numNodes: numNodes, passes: map[[2]int]bool{}}
	for e := range numEdges {
		from := rapid.IntRange(0, numNodes-1).Draw(t, fmt.Sprintf("edge%dFrom", e))
		to := rapid.IntRange(0, numNodes-1).Draw(t, fmt.Sprintf("edge%dTo", e))
		spec.edges = append(spec.edges, [2]int{from, to})
		for c := range numCursors {
			if rapid.Bool().Draw(t, fmt.Sprintf("edge%dPasses%d", e, c)) {
				spec.passes[[2]int{e, c}] = true
			}
		}
	}
	for c := range numCursors {
		spec.cursors = append(spec.cursors, rapid.IntRange(0, numNodes-1).Draw(t, fmt.Sprintf("cursor%dAt", c)))
	}
	return spec
}

type runResult struct {
	log        []string // every condition ask and node entry, in order
	violations []string
	err        error
	final      map[int]int // cursor value -> node index at quiescence
}

// Build the spec into a machine and run one Progress over it with SyncRunner, recording every
// callback. Visits are derived from entry calls: a cursor's visit count bumps each time it enters a
// node (initial placement is visit 0).
func runSpec(spec machineSpec) *runResult {
	res := &runResult{final: map[int]int{}}
	machine := fsa.New[int]()

	visits := make([]int, len(spec.cursors))
	nodes := make([]fsa.Node, spec.numNodes)
	nodeIndex := map[fsa.Node]int{}
	for i := range nodes {
		nodes[i] = machine.NewNode(func(_ context.Context, _ fsa.FSA[int], _ fsa.Edge, v int) error {
			visits[v]++
			res.log = append(res.log, fmt.Sprintf("enter n%d c%d", i, v))
			return nil
		})
		nodeIndex[nodes[i]] = i
	}

	budget := maps.Clone(spec.passes)
	asked := map[[3]int]bool{} // (edge, cursor, visit)
	for e, ft := range spec.edges {
		machine.NewEdge(func(_ context.Context, _ fsa.FSA[int], v int) (fsa.ConditionResult, error) {
			key := [3]int{e, v, visits[v]}
			if asked[key] {
				res.violations = append(res.violations,
					fmt.Sprintf("edge %d asked twice for cursor %d during visit %d", e, v, visits[v]))
			}
			asked[key] = true
			if budget[[2]int{e, v}] {
				budget[[2]int{e, v}] = false
				res.log = append(res.log, fmt.Sprintf("ask e%d c%d visit%d -> pass", e, v, visits[v]))
				return fsa.ConditionPass, nil
			}
			res.log = append(res.log, fmt.Sprintf("ask e%d c%d visit%d -> fail", e, v, visits[v]))
			return fsa.ConditionFail, nil
		}, nodes[ft[0]], nodes[ft[1]])
	}

	for c, at := range spec.cursors {
		machine.NewCursor(c, nodes[at])
	}

	res.err = machine.Progress(context.Background(), fsa.SyncRunner)
	for v, n := range maps.Collect(machine.Cursors) {
		res.final[v] = nodeIndex[n]
	}
	return res
}

func checkOutcome(t *rapid.T, res *runResult) {
	if res.err != nil {
		// A race between two arrivals is a legal outcome of an arbitrary graph; anything else is not.
		require.ErrorContains(t, res.err, "both moving")
	}
	require.Empty(t, res.violations)
}

// Within one visit to a node, each of the node's conditions is asked at most once per cursor. Only
// re-entering the node (a move) re-samples them.
func TestRapidConditionOncePerVisit(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		checkOutcome(t, runSpec(genSpec(t)))
	})
}

// Under SyncRunner, Progress is fully deterministic: two runs of the same machine produce identical
// callback sequences and identical outcomes.
func TestRapidDeterministic(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		spec := genSpec(t)
		first := runSpec(spec)
		second := runSpec(spec)
		checkOutcome(t, first)

		require.Equal(t, first.log, second.log)
		require.Equal(t, fmt.Sprint(first.err), fmt.Sprint(second.err))
		require.Equal(t, first.final, second.final)
	})
}
