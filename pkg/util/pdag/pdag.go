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

// Package pdag provides facilities for constructing a DAG and the traversing it
// efficiently in parallel.
package pdag

import (
	"context"
	"errors"
	"iter"
	"sync"

	"golang.org/x/sync/errgroup"
)

func New[T any]() *DAG[T] {
	return &DAG[T]{m: *sync.NewCond(&sync.Mutex{})}
}

type DAG[T any] struct {
	m     sync.Cond
	nodes []node[T]
}

type node[T any] struct {
	v T

	// The direct prerequisites of the node.
	//
	// For example given a graph
	//
	//	A -> B -> C
	//
	// Node A would have no prerequisites, B would have [A] and C would have [B].
	prerequisites []int

	status status
}

// Create a new node. The node will not be processed until Done is called, and all nodes
// with edges leading to it have been processed.
func (g *DAG[T]) NewNode(v T) (Node, Done) {
	g.m.L.Lock()
	defer g.m.L.Unlock()
	idx := len(g.nodes)
	g.nodes = append(g.nodes, node[T]{v: v})
	return Node{idx}, func() { g.markReady(idx) }
}

func (g *DAG[T]) markReady(idx int) {
	g.m.L.Lock()
	defer g.m.L.Unlock()
	if g.nodes[idx].status != preparing {
		// The node has already been marked ready.
		return
	}
	g.nodes[idx].status = ready
	g.m.Broadcast()
}

func (g *DAG[T]) markDone(idx int) {
	g.m.L.Lock()
	defer g.m.L.Unlock()
	g.nodes[idx].status = done
	g.m.Broadcast()
}

type ErrorCycle[T any] struct {
	Cycle []T
}

func (err ErrorCycle[T]) Error() string {
	return "Cycle found"
}

// ErrorReentrant indicates that [NewEdge] attempted to create a "from->to" connection
// where "to" was already walked (or currently being walked) and "from" was not yet walked
// (or currently is being walked). This would violate the [DAG]s traversal guarantees,
// since it guarantees that nodes will only be seen after all of their from edges have
// been seen.
type ErrorReentrant struct{}

func (err ErrorReentrant) Error() string {
	return "connection is re-entrant"
}

type status int

const (
	preparing  = iota // A node is still having it's dependencies defined
	ready             // A node is ready to be processed
	processing        // A node is currently being processed
	done              // A node has been processed successfully
)

// NewEdge creates a new edge FROM -> TO, ensuring that FROM comes before TO in a traversal of g.
func (g *DAG[T]) NewEdge(from, to Node) error {
	// Self-loops are a no-op
	if from.i == to.i {
		return nil
	}

	g.m.L.Lock() // Ensure that the graph shape doesn't change while we verify that our new edge is OK.
	defer g.m.L.Unlock()

	switch g.nodes[to.i].status {
	case processing, done:
		if g.nodes[from.i].status != done {
			return ErrorReentrant{}
		}
	}

	// Check if adding edge from->to would create a cycle by checking if there's already
	// a path from 'to' to 'from' in the existing graph.
	// Since we store prerequisites (incoming edges), we need to traverse backwards from 'from'
	// to see if we can reach 'to'. If we can, then adding 'from->to' would create a cycle.

	visited := make([]bool, len(g.nodes))
	parent := make([]int, len(g.nodes))
	for i := range parent {
		parent[i] = -1
	}

	// Stack-based DFS from 'from' following prerequisite edges backwards
	stack := []int{from.i}
	visited[from.i] = true

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		for _, prereq := range g.nodes[current].prerequisites {
			if prereq == to.i {
				// Cycle detected: there's already a path from 'from' to 'to' via prerequisites.
				// Adding 'from->to' would complete the cycle.
				// Reconstruct cycle as: from -> to -> ... -> current

				// Build path from 'to' to 'current' (via parent pointers back through from)
				pathFromTo := []T{g.nodes[to.i].v}

				for node := current; node != from.i; {
					pathFromTo = append(pathFromTo, g.nodes[node].v)
					node = parent[node]
				}

				// Build cycle starting from 'from' following the path
				cycle := []T{g.nodes[from.i].v}
				cycle = append(cycle, pathFromTo...)
				return ErrorCycle[T]{Cycle: cycle}
			}

			if !visited[prereq] {
				visited[prereq] = true
				parent[prereq] = current
				stack = append(stack, prereq)
			}
		}
	}

	// No cycle detected, we can now update the prerequisites to include the new edge.
	// Since from->to, 'to' has 'from' as a prerequisite.

	g.nodes[to.i].prerequisites = append(g.nodes[to.i].prerequisites, from.i)

	return nil
}

// Drain nodes from the dag. Nodes will always come after their dependents.
//
// Any returned Done func **must** be called. Failing to do so may cause other iterators
// to block indefinitely or leak memory.
//
// The returned iterator may be called in parallel. When called in parallel, the iterator
// may block when no nodes are available. The returned iterator will not return until the
// entire graph has been walked.
//
// Drain will stop the iterator when the passed in ctx is canceled. If the caller is
// concerned about the context being canceled, they **must** check ctx.Err() to determine
// if Drain finished.
func (g *DAG[T]) Drain(ctx context.Context) iter.Seq2[T, Done] {
	// should wait on a broadcast if no nodes are found
	//
	// the lock for g.m.L should be held when findNextNode is called.
	findNextNode := func() int {
		var pending bool
		for {
		search:
			for idx, node := range g.nodes {
				switch node.status {
				case ready:
					for _, prereq := range node.prerequisites {
						if g.nodes[prereq].status != done {
							continue search
						}
					}
					// All pre-requsites are done, so node is safe to work on.
					return idx
				case preparing, processing:
					pending = true
					continue
				case done:
					continue
				default:
					panic("impossible - there are only 4 valid values")
				}
			}

			if pending {
				pending = false
				g.m.Wait()
			} else {
				return -1
			}
		}
	}

	iterate := func(yield func(T, Done) bool) bool {
		g.m.L.Lock()

		node := findNextNode()
		if node == -1 {
			// No nodes are available, so we are done
			g.m.L.Unlock()
			return false
		}
		// Mark the node as processing so it won't be picked up again
		g.nodes[node].status = processing
		v := g.nodes[node].v
		g.m.L.Unlock()

		return yield(v, func() { g.markDone(node) })
	}

	return func(yield func(T, Done) bool) {
		for iterate(yield) {
		}
	}
}

// Calling marks a process as done.
type Done = func()

// Walk the DAG in parallel, blocking until all nodes have been processed or an error is
// returned.
//
// [Node]s added during a Walk will be observed during the walk.
func (g *DAG[T]) Walk(ctx context.Context, process func(context.Context, T) error, options ...WalkOption) error {
	var opts walkOptions
	for _, o := range options {
		o.applyWalkOption(&opts)
	}

	wg, ctx := errgroup.WithContext(ctx)
	if opts.maxProcs > 0 {
		wg.SetLimit(opts.maxProcs)
	}

	var cancelError error
	for node, done := range g.Drain(ctx) {
		if err := ctx.Err(); err != nil {
			cancelError = err
			break
		}
		wg.Go(func() error {
			defer done()
			return process(ctx, node)
		})
	}

	return errors.Join(wg.Wait(), cancelError)
}

// Node is a reference to a node in a [DAG].
//
// The only way to create a Node is with [DAG.NewNode], and the Node is only valid in the
// context of the graph it was created in.
type Node struct {
	i int // An index into the DAG the node is registered with
}

type walkOptions struct {
	maxProcs int
}

type WalkOption interface {
	applyWalkOption(*walkOptions)
}

type walkOption func(*walkOptions)

func (f walkOption) applyWalkOption(o *walkOptions) { f(o) }

// MaxProcs limits the number of concurrent work threads in a [DAG.Walk] call.
//
// i is ignored if it is less then 1.
func MaxProcs(i int) WalkOption {
	return walkOption(func(o *walkOptions) { o.maxProcs = i })
}
