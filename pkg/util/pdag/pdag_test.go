// Copyright 2025, Pulumi Corporation.
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

package pdag_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func extractFromDAG[T any](t *testing.T, dag pdag.DAG[T]) []T {
	var actualElems []T
	require.NoError(t, dag.Walk(t.Context(), func(_ context.Context, v T) error {
		actualElems = append(actualElems, v)
		return nil
	}, pdag.MaxProcs(1)))
	return actualElems
}

func TestDAG_NewNode(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]
	dag.NewNode("node1")
	dag.NewNode("node2")
	dag.NewNode("node3")

	assert.ElementsMatch(t, extractFromDAG(t, dag), []string{
		"node1", "node2", "node3",
	})
}

func TestDAG_NewEdge_Simple(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]
	h1 := dag.NewNode("node1")
	h2 := dag.NewNode("node2")

	err := dag.NewEdge(h2, h1)
	require.NoError(t, err)

	assert.Equal(t, extractFromDAG(t, dag), []string{
		// Node2 must come first, since node1 depends on it
		"node2", "node1",
	})
}

func TestDAG_NewEdge_MultipleEdges(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]
	a := dag.NewNode("a")
	b := dag.NewNode("b")
	c := dag.NewNode("c")

	require.NoError(t, dag.NewEdge(b, a))
	require.NoError(t, dag.NewEdge(b, c))
	require.NoError(t, dag.NewEdge(a, c))

	assert.Equal(t, extractFromDAG(t, dag), []string{
		"b", "a", "c",
	})
}

func testEdgeCycle[T comparable](t *testing.T, graph []struct{ to, from T }, expectedCycle []T) {
	var dag pdag.DAG[T]
	handles := map[T]pdag.Node{}
	for _, v := range graph {
		toH, ok := handles[v.to]
		if !ok {
			toH = dag.NewNode(v.to)
			handles[v.to] = toH
		}
		fromH, ok := handles[v.from]
		if !ok {
			fromH = dag.NewNode(v.from)
			handles[v.from] = fromH
		}
		err := dag.NewEdge(fromH, toH)
		if err != nil {
			var errC pdag.ErrorCycle[T]
			require.ErrorAs(t, err, &errC)

			// errC.Cycle matches expectedCycle if a rotation of errC.Cycle is
			// equal to expectedCycle.
			for rotation := range rotationsOf(expectedCycle) {
				if slices.Equal(rotation, errC.Cycle) {
					return
				}
			}

			slices.Reverse(errC.Cycle)

			require.Equal(t, expectedCycle, errC.Cycle,
				"A cycle was detected but doesn't match the expected cycle under rotation")
			return
		}
	}
	require.FailNow(t, "No cycle detected")
}

func rotationsOf[E any](arr []E) iter.Seq[[]E] {
	return func(yield func([]E) bool) {
		if len(arr) == 0 {
			return
		}

		// Allocate one slice that will be reused for all rotations
		rotation := make([]E, len(arr))

		for i := range arr {
			// Build rotation starting at index i
			for j := range arr {
				rotation[j] = arr[(i+j)%len(arr)]
			}

			if !yield(rotation) {
				return
			}
		}
	}
}

func TestDAG_NewEdge_DetectsCycle(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"node2", "node1"},
		{"node1", "node2"}, // This creates the cycle
	}, []string{"node2", "node1"})
}

func TestDAG_NewEdge_DetectsIndirectCycle(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"node2", "node1"},
		{"node3", "node2"},
		{"node1", "node3"}, // This creates the cycle
	}, []string{"node3", "node1", "node2"})
}

func TestDAG_NewEdge_SelfLoop(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]
	h1 := dag.NewNode("node1")

	// Self-loop should be a no-op (no error)
	err := dag.NewEdge(h1, h1)
	require.NoError(t, err)

	// Verify the node still exists and can be walked
	assert.Equal(t, []string{"node1"}, extractFromDAG(t, dag))
}

func TestDAG_NewEdge_DuplicateEdge(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]
	h1 := dag.NewNode("node1")
	h2 := dag.NewNode("node2")

	err := dag.NewEdge(h1, h2)
	require.NoError(t, err)

	// Adding the same edge again should succeed (idempotent)
	err = dag.NewEdge(h1, h2)
	require.NoError(t, err)

	// Verify the walk order is still correct
	assert.Equal(t, []string{"node1", "node2"}, extractFromDAG(t, dag))
}

func TestDAG_Walk_EmptyDAG(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[struct{}]

	err := dag.Walk(t.Context(), func(context.Context, struct{}) error {
		t.Fatal("should not be called")
		return nil
	})

	require.NoError(t, err)
}

func TestDAG_Walk_TwoIndependentNodes(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[int]
	for range 20 {
		dag.NewNode(0)
	}

	var count atomic.Int32

	err := dag.Walk(t.Context(), func(ctx context.Context, v int) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, 20, int(count.Load()))
}

func TestDAG_Walk_WithDependency(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[int32]
	h0 := dag.NewNode(0)
	h1 := dag.NewNode(1)
	h2 := dag.NewNode(2)

	require.NoError(t, dag.NewEdge(h0, h1))
	require.NoError(t, dag.NewEdge(h1, h2))

	var expectedNext atomic.Int32
	err := dag.Walk(t.Context(), func(ctx context.Context, v int32) error {
		require.True(t, expectedNext.CompareAndSwap(v, v+1))
		return nil
	}, pdag.MaxProcs(4))
	require.NoError(t, err)
	assert.Equal(t, 3, int(expectedNext.Load()))
}

func TestDAG_Walk_ErrorPropogation(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[int]
	h0 := dag.NewNode(0)
	h1 := dag.NewNode(1)
	h2 := dag.NewNode(2)

	// Create a chain: 0 -> 1 -> 2
	require.NoError(t, dag.NewEdge(h0, h1))
	require.NoError(t, dag.NewEdge(h1, h2))

	var completed atomic.Int32
	expectedErr := errors.New("processing error")

	err := dag.Walk(t.Context(), func(ctx context.Context, v int) error {
		completed.Add(1)
		if v == 1 {
			// Error on the middle node after the first node has completed
			return expectedErr
		}
		return nil
	})

	assert.ErrorIs(t, err, expectedErr)
	// Node 0 should have completed, node 1 errored (but was added), node 2 should not run
	assert.Equal(t, int(completed.Load()), 2)
}

func TestDAG_Walk_ContextCancellation(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]
	dag.NewNode("node1")

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	err := dag.Walk(ctx, func(ctx context.Context, v string) error {
		return ctx.Err()
	})

	assert.ErrorIs(t, err, context.Canceled)
}

// TestDAG_Walk_MaxProcs checks that we saturate up to the maximum allows procs, but no
// further.
//
// We do this by constructing a graph that must be walked in phases of 4, 4, 2 and 1 with
// concurrency 4. We then assert that we have that many concurrent threads for each phase,
// and no more.
func TestDAG_Walk_MaxProcs(t *testing.T) {
	t.Parallel()

	var dag pdag.DAG[string]

	const maxConcurrency = 4

	r := dag.NewNode("root")
	a := dag.NewNode("a")
	b := dag.NewNode("b")
	a1, a2, a3 := dag.NewNode("a1"), dag.NewNode("a2"), dag.NewNode("a3")
	b1, b2, b3 := dag.NewNode("b1"), dag.NewNode("b2"), dag.NewNode("b3")
	s1, s2 := dag.NewNode("s1"), dag.NewNode("s2")

	require.NoError(t, errors.Join(
		dag.NewEdge(a, r),
		dag.NewEdge(b, r),

		dag.NewEdge(a1, a),
		dag.NewEdge(a2, a),
		dag.NewEdge(a3, a),

		dag.NewEdge(b1, b),
		dag.NewEdge(b2, b),
		dag.NewEdge(b3, b),

		dag.NewEdge(s1, a),
		dag.NewEdge(s2, a),

		dag.NewEdge(s1, b),
		dag.NewEdge(s2, b),
	))

	walkDone := make(chan struct{})

	var currentPhase atomic.Int32
	phases := []*struct {
		expected int32
		actual   atomic.Int32
		done     chan struct{}
	}{
		{expected: 4},
		{expected: 4},
		{expected: 2},
		{expected: 1},
	}
	for i := range phases {
		phases[i].done = make(chan struct{})
	}

	recv := make(chan struct{})

	go func() {
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			phase := phases[currentPhase.Load()]
			phase.actual.Add(1)

			recv <- struct{}{} // Indicate that work is being done
			<-phase.done       // Wait to indicate that the phase is over

			return nil
		}, pdag.MaxProcs(maxConcurrency))
		require.NoError(t, err)
		close(walkDone)
	}()

	//nolint:paralleltest // Phases must run in sequence
	for i, phase := range phases {
		t.Run(fmt.Sprintf("phase-%d", i), func(t *testing.T) {
			for range phase.expected {
				<-recv
			}
			assert.Equal(t, phase.expected, phase.actual.Load())
			currentPhase.Add(1)
			close(phase.done)
		})
	}

	<-walkDone
}

func TestDAG_DiamondDependency(t *testing.T) {
	var dag pdag.DAG[string]
	h1 := dag.NewNode("root")
	h2 := dag.NewNode("left")
	h3 := dag.NewNode("right")
	h4 := dag.NewNode("bottom")

	// Create diamond: root -> left -> bottom, root -> right -> bottom
	require.NoError(t, dag.NewEdge(h2, h1))
	require.NoError(t, dag.NewEdge(h3, h1))
	require.NoError(t, dag.NewEdge(h4, h2))
	require.NoError(t, dag.NewEdge(h4, h3))

	// Depending on the order that functions execute, we can guarantee that we see
	// right before left or vice versa.

	t.Run("left before right", func(t *testing.T) {
		t.Parallel()

		found := []string{}
		leftDone := make(chan struct{})
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			switch v {
			case "left":
				found = append(found, v)
				close(leftDone)
			case "right":
				<-leftDone
				found = append(found, v)
			default:
				found = append(found, v)
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"bottom", "left", "right", "root"}, found)
	})

	t.Run("right before left", func(t *testing.T) {
		t.Parallel()

		found := []string{}
		rightDone := make(chan struct{})
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			switch v {
			case "right":
				found = append(found, v)
				close(rightDone)
			case "left":
				<-rightDone
				found = append(found, v)
			default:
				found = append(found, v)
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"bottom", "right", "left", "root"}, found)
	})
}

func TestDAG_ComplexTopology(t *testing.T) {
	t.Parallel()

	dag := &pdag.DAG[int]{}
	h1 := dag.NewNode(1)
	h2 := dag.NewNode(2)
	h3 := dag.NewNode(3)
	h4 := dag.NewNode(4)
	h5 := dag.NewNode(5)

	// Create: 1->2, 1->3, 2->4, 3->4, 4->5
	err := dag.NewEdge(h1, h2)
	require.NoError(t, err)
	err = dag.NewEdge(h1, h3)
	require.NoError(t, err)
	err = dag.NewEdge(h2, h4)
	require.NoError(t, err)
	err = dag.NewEdge(h3, h4)
	require.NoError(t, err)
	err = dag.NewEdge(h4, h5)
	require.NoError(t, err)

	var executed atomic.Int32

	err = dag.Walk(t.Context(), func(ctx context.Context, v int) error {
		executed.Add(1)
		return nil
	})

	require.NoError(t, err)

	// All 5 nodes should execute
	assert.Equal(t, int32(5), executed.Load())
}

// BenchmarkDAG_NewEdge_DenseGraph benchmarks building a dense DAG to demonstrate
// the performance improvement from O(E²) to O(V+E) cycle detection.
func BenchmarkDAG_NewEdge_DenseGraph(b *testing.B) {
	sizes := []int{50, 100, 200}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				var dag pdag.DAG[int]
				nodes := make([]pdag.Node, size)
				for j := 0; j < size; j++ {
					nodes[j] = dag.NewNode(j)
				}
				b.StartTimer()

				// Create a chain: 0 -> 1 -> 2 -> ... -> size-1
				// Then add cross edges from each node to several later nodes
				// This creates a dense graph without cycles
				for j := 0; j < size-1; j++ {
					err := dag.NewEdge(nodes[j], nodes[j+1])
					if err != nil {
						b.Fatal(err)
					}
					// Add edges to multiple future nodes (skip some to avoid cycles)
					for k := j + 2; k < size && k < j+10; k++ {
						err := dag.NewEdge(nodes[j], nodes[k])
						if err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}

// BenchmarkDAG_NewEdge_CycleDetection benchmarks cycle detection performance
// by attempting to add an edge that would create a cycle in graphs of varying sizes.
func BenchmarkDAG_NewEdge_CycleDetection(b *testing.B) {
	sizes := []int{50, 100, 200, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			// Build a long chain once
			var dag pdag.DAG[int]
			nodes := make([]pdag.Node, size)
			for j := 0; j < size; j++ {
				nodes[j] = dag.NewNode(j)
			}
			// Create chain: 0 -> 1 -> 2 -> ... -> size-1
			for j := 0; j < size-1; j++ {
				err := dag.NewEdge(nodes[j], nodes[j+1])
				if err != nil {
					b.Fatal(err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Try to add an edge that would create a cycle
				// This should be detected quickly with O(V+E) complexity
				err := dag.NewEdge(nodes[size-1], nodes[0])
				if err == nil {
					b.Fatal("Expected cycle error")
				}
			}
		})
	}
}
