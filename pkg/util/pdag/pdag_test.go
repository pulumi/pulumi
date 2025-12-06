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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFinishedNode[T any](g *pdag.DAG[T], v T) pdag.Node {
	node, done := g.NewNode(v)
	done()
	return node
}

func extractFromDAG[T any](t *testing.T, dag *pdag.DAG[T]) []T {
	var actualElems []T
	require.NoError(t, dag.Walk(t.Context(), func(_ context.Context, v T) error {
		actualElems = append(actualElems, v)
		return nil
	}, pdag.MaxProcs(1)))
	return actualElems
}

func TestDAG_NewNode(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	newFinishedNode(dag, "node1")
	newFinishedNode(dag, "node2")
	newFinishedNode(dag, "node3")

	assert.ElementsMatch(t, extractFromDAG(t, dag), []string{
		"node1", "node2", "node3",
	})
}

func TestDAG_NewEdge_Simple(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	h1 := newFinishedNode(dag, "node1")
	h2 := newFinishedNode(dag, "node2")

	err := dag.NewEdge(h2, h1)
	require.NoError(t, err)

	assert.Equal(t, extractFromDAG(t, dag), []string{
		// Node2 must come first, since node1 depends on it
		"node2", "node1",
	})
}

func TestDAG_NewEdge_MultipleEdges(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	a := newFinishedNode(dag, "a")
	b := newFinishedNode(dag, "b")
	c := newFinishedNode(dag, "c")

	require.NoError(t, dag.NewEdge(b, a))
	require.NoError(t, dag.NewEdge(b, c))
	require.NoError(t, dag.NewEdge(a, c))

	assert.Equal(t, extractFromDAG(t, dag), []string{
		"b", "a", "c",
	})
}

func testEdgeCycle[T comparable](t *testing.T, graph []struct{ to, from T }, expectedCycle []T) {
	dag := pdag.New[T]()
	handles := map[T]pdag.Node{}
	for _, v := range graph {
		toH, ok := handles[v.to]
		if !ok {
			toH = newFinishedNode(dag, v.to)
			handles[v.to] = toH
		}
		fromH, ok := handles[v.from]
		if !ok {
			fromH = newFinishedNode(dag, v.from)
			handles[v.from] = fromH
		}
		err := dag.NewEdge(fromH, toH)
		if err != nil {
			var errC pdag.ErrorCycle[T]
			require.ErrorAs(t, err, &errC)
			require.Equal(t, expectedCycle, errC.Cycle)
			return
		}
	}
	require.FailNow(t, "No cycle detected")
}

func TestDAG_NewEdge_DetectsCycle(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"node2", "node1"},
		{"node1", "node2"},
	}, []string{"node2", "node1"})
}

func TestDAG_NewEdge_CycleFormat(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"b", "a"},
		{"a", "b"},
	}, []string{"b", "a"})
}

func TestDAG_NewEdge_ComplexCycle(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"c", "a"},
		{"a", "b"},
		{"b", "a"},
	}, []string{"a", "b"})
}

func TestDAG_NewEdge_DetectsIndirectCycle(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"node2", "node1"},
		{"node3", "node2"},
		{"node1", "node3"},
	}, []string{"node3", "node1", "node2"})
}

func TestDAG_NewEdge_LargeCycle(t *testing.T) {
	t.Parallel()

	testEdgeCycle(t, []struct{ to, from string }{
		{"b", "a"},
		{"c", "b"},
		{"d", "c"},
		{"e", "d"},
		{"a", "e"},
	}, []string{"e", "a", "b", "c", "d"})
}

func TestDAG_NewEdge_SelfLoop(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	h1 := newFinishedNode(dag, "node1")

	// Self-loop should be a no-op (no error)
	err := dag.NewEdge(h1, h1)
	require.NoError(t, err)

	// Verify the node still exists and can be walked
	assert.Equal(t, []string{"node1"}, extractFromDAG(t, dag))
}

func TestDAG_NewEdge_DuplicateEdge(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	h1 := newFinishedNode(dag, "node1")
	h2 := newFinishedNode(dag, "node2")

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

	dag := pdag.New[struct{}]()

	err := dag.Walk(t.Context(), func(context.Context, struct{}) error {
		t.Fatal("should not be called")
		return nil
	})

	require.NoError(t, err)
}

func TestDAG_Walk_TwoIndependentNodes(t *testing.T) {
	t.Parallel()

	dag := pdag.New[int]()
	for range 20 {
		newFinishedNode(dag, 0)
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

	dag := pdag.New[int32]()
	h0 := newFinishedNode(dag, 0)
	h1 := newFinishedNode(dag, 1)
	h2 := newFinishedNode(dag, 2)

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

	dag := pdag.New[int]()
	h0 := newFinishedNode(dag, 0)
	h1 := newFinishedNode(dag, 1)
	h2 := newFinishedNode(dag, 2)

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

// TestDAG_Walk_MaxProcs checks that we saturate up to the maximum allows procs, but no
// further.
//
// We do this by constructing a graph that must be walked in phases of 4, 4, 2 and 1 with
// concurrency 4. We then assert that we have that many concurrent threads for each phase,
// and no more.
func TestDAG_Walk_MaxProcs(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()

	const maxConcurrency = 4

	r := newFinishedNode(dag, "root")
	a := newFinishedNode(dag, "a")
	b := newFinishedNode(dag, "b")
	a1, a2, a3 := newFinishedNode(dag, "a1"), newFinishedNode(dag, "a2"), newFinishedNode(dag, "a3")
	b1, b2, b3 := newFinishedNode(dag, "b1"), newFinishedNode(dag, "b2"), newFinishedNode(dag, "b3")
	s1, s2 := newFinishedNode(dag, "s1"), newFinishedNode(dag, "s2")

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
	dag := func() *pdag.DAG[string] {
		dag := pdag.New[string]()
		h1 := newFinishedNode(dag, "root")
		h2 := newFinishedNode(dag, "left")
		h3 := newFinishedNode(dag, "right")
		h4 := newFinishedNode(dag, "bottom")

		// Create diamond: root -> left -> bottom, root -> right -> bottom
		require.NoError(t, dag.NewEdge(h2, h1))
		require.NoError(t, dag.NewEdge(h3, h1))
		require.NoError(t, dag.NewEdge(h4, h2))
		require.NoError(t, dag.NewEdge(h4, h3))

		return dag
	}

	// Depending on the order that functions execute, we can guarantee that we see
	// right before left or vice versa.

	t.Run("left before right", func(t *testing.T) {
		t.Parallel()

		found := []string{}
		leftDone := make(chan struct{})
		err := dag().Walk(t.Context(), func(ctx context.Context, v string) error {
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
		err := dag().Walk(t.Context(), func(ctx context.Context, v string) error {
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

	dag := pdag.New[int]()
	h1 := newFinishedNode(dag, 1)
	h2 := newFinishedNode(dag, 2)
	h3 := newFinishedNode(dag, 3)
	h4 := newFinishedNode(dag, 4)
	h5 := newFinishedNode(dag, 5)

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
				dag := pdag.New[int]()
				nodes := make([]pdag.Node, size)
				for j := 0; j < size; j++ {
					nodes[j] = newFinishedNode(dag, j)
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
			dag := pdag.New[int]()
			nodes := make([]pdag.Node, size)
			for j := 0; j < size; j++ {
				nodes[j] = newFinishedNode(dag, j)
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

// TestNewNode_BlocksUntilDone tests that nodes are not processed until Done is called
func TestNewNode_BlocksUntilDone(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	n1, done1 := dag.NewNode("n1")
	n2, done2 := dag.NewNode("n2")

	require.NoError(t, dag.NewEdge(n1, n2))

	// Leave both nodes unfinished - don't call done1() or done2() yet

	n1Done := make(chan struct{})
	n2Done := make(chan struct{})
	walkDone := make(chan struct{})

	go func() { // Walk will be blocking, so it must be in a separate thread
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			switch v {
			case "n1":
				close(n1Done)
			case "n2":
				close(n2Done)
			}
			return nil
		})
		require.NoError(t, err)
		close(walkDone)
	}()

	// Make sure that no nodes will be processed before we call close.
	select {
	case <-n1Done:
		t.Fatal("n1 should not be processed before done1() is called")
	case <-n2Done:
		t.Fatal("n2 should not be processed before done1() is called")
	case <-time.After(100 * time.Millisecond): // no nodes processed
	}

	done1()

	select {
	case <-n1Done: // n1 was processed
	case <-time.After(1 * time.Second):
		t.Fatal("n1 should be processed after done1() is called")
	}

	// n2 is still not done, so we need to wait
	select {
	case <-n2Done:
		t.Fatal("n2 should not be processed before done2() is called")
	case <-time.After(100 * time.Millisecond): // Wait
	}

	done2()
	<-n2Done   // Make sure that n2 was processed
	<-walkDone // Ensure the walk function returns
}

// TestNewNode_DoneIsIdempotent tests that Done can be called multiple times safely
func TestNewNode_DoneIsIdempotent(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	_, done := dag.NewNode("node")

	done()
	done()
	done()

	var processCount atomic.Int32
	err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
		processCount.Add(1)
		return nil
	})

	require.NoError(t, err)
	assert.EqualValues(t, 1, processCount.Load(), "node should be processed exactly once")
}

// TestNewNode_WaitsForPrerequisiteDone tests that nodes wait for prerequisite Done even if their own Done is called
func TestNewNode_WaitsForPrerequisiteDone(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	n1, done1 := dag.NewNode("n1")
	n2, done2 := dag.NewNode("n2")

	require.NoError(t, dag.NewEdge(n1, n2))

	done2()

	n1Done := make(chan struct{})
	n2Done := make(chan struct{})
	walkDone := make(chan struct{})

	go func() {
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			switch v {
			case "n1":
				close(n1Done)
			case "n2":
				close(n2Done)
			}
			return nil
		})
		require.NoError(t, err)
		close(walkDone)
	}()

	// Verify that no nodes are processed even though done2() was called
	select {
	case <-n1Done:
		t.Fatal("n1 should not be processed before done1() is called")
	case <-n2Done:
		t.Fatal("n2 should not be processed before done1() is called (even though done2() was called)")
	case <-time.After(100 * time.Millisecond):
	}

	// Now call done1 to unblock the walk
	done1()

	<-n1Done
	<-n2Done
	<-walkDone
}

// TestNewEdge_ErrorReentrant tests that NewEdge returns ErrorReentrant appropriately
func TestNewEdge_ErrorReentrant(t *testing.T) {
	t.Parallel()

	t.Run("preparing to processing", func(t *testing.T) {
		t.Parallel()

		dag := pdag.New[string]()
		n1, done1 := dag.NewNode("n1")
		done1()

		// Start walk to put n1 in processing state
		n1Processing := make(chan struct{})
		n1CanFinish := make(chan struct{})

		go func() {
			err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
				close(n1Processing)
				<-n1CanFinish
				return nil
			})
			require.NoError(t, err)
		}()

		<-n1Processing // Wait for n1 to be processing

		// Try to add edge from preparing node to processing n1
		n2, _ := dag.NewNode("n2") // n2 is in preparing state (done not called)
		err := dag.NewEdge(n2, n1)

		var errReentrant pdag.ErrorReentrant
		assert.ErrorAs(t, err, &errReentrant)

		close(n1CanFinish)
	})

	t.Run("preparing to done", func(t *testing.T) {
		t.Parallel()

		dag := pdag.New[string]()
		n1, done1 := dag.NewNode("n1")
		done1()

		// Walk n1 so it becomes done
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			return nil
		})
		require.NoError(t, err)

		// Try to add edge from preparing node to done n1
		n2, _ := dag.NewNode("n2") // n2 is in preparing state
		err = dag.NewEdge(n2, n1)

		var errReentrant pdag.ErrorReentrant
		assert.ErrorAs(t, err, &errReentrant)
	})

	t.Run("done to processing", func(t *testing.T) {
		t.Parallel()

		dag := pdag.New[string]()
		n1, done1 := dag.NewNode("n1")
		done1()

		// Walk n1 to completion
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			return nil
		})
		require.NoError(t, err)

		// n1 is now done. Create n2 and start processing it
		n2, done2 := dag.NewNode("n2")
		done2()

		n2Processing := make(chan struct{})
		n2CanFinish := make(chan struct{})

		go func() {
			err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
				close(n2Processing)
				<-n2CanFinish
				return nil
			})
			require.NoError(t, err)
		}()

		<-n2Processing // Wait for n2 to be processing

		// Try to add edge from done n1 to processing n2
		err = dag.NewEdge(n1, n2)
		require.NoError(t, err, "should allow edge from done to processing")

		close(n2CanFinish)
	})

	t.Run("done to done", func(t *testing.T) {
		t.Parallel()

		dag := pdag.New[string]()
		n1, done1 := dag.NewNode("n1")
		n2, done2 := dag.NewNode("n2")
		done1()
		done2()

		// Walk both to completion
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			return nil
		})
		require.NoError(t, err)

		// Both are done, should allow edge
		err = dag.NewEdge(n1, n2)
		require.NoError(t, err, "should allow edge from done to done")
	})
}

func TestWalk_DynamicNodeAddition(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()

	root, doneRoot := dag.NewNode("root")
	doneRoot()

	// Track processing order
	var processed []string
	var processingMu sync.Mutex

	// Channels to coordinate test
	rootProcessingStarted := make(chan struct{})
	childrenReady := make(chan struct{})
	walkComplete := make(chan struct{})

	go func() {
		err := dag.Walk(t.Context(), func(ctx context.Context, v string) error {
			processingMu.Lock()
			processed = append(processed, v)
			processingMu.Unlock()

			if v == "root" {
				close(rootProcessingStarted)
				<-childrenReady
			}
			return nil
		})
		require.NoError(t, err)
		close(walkComplete)
	}()

	<-rootProcessingStarted

	child1, doneChild1 := dag.NewNode("child1")
	child2, doneChild2 := dag.NewNode("child2")
	close(childrenReady)

	require.NoError(t, dag.NewEdge(root, child1))
	require.NoError(t, dag.NewEdge(child1, child2))

	doneChild1()
	doneChild2()

	<-walkComplete

	assert.Equal(t, []string{"root", "child1", "child2"}, processed)
}

func TestWalk_ContextCancellation(t *testing.T) {
	t.Parallel()

	dag := pdag.New[string]()
	n1, done1 := dag.NewNode("n1")
	n2, done2 := dag.NewNode("n2")
	require.NoError(t, dag.NewEdge(n1, n2))
	done1()
	done2()

	ctx, cancel := context.WithCancel(t.Context())

	err := dag.Walk(ctx, func(ctx context.Context, v string) error {
		switch v {
		case "n1":
			cancel()
		case "n2":
			assert.Fail(t, "n2 should never be processed")
		}
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
}
