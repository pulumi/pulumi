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
	"iter"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

type DAG[T any] struct{ nodes []node[T] }

type node[T any] struct {
	v T
	// the list edge destinations for edges originating from this node.
	to []int
}

func (g *DAG[T]) NewNode(v T) Node {
	g.nodes = append(g.nodes, node[T]{v: v})
	return Node{len(g.nodes) - 1}
}

type ErrorCycle[T any] struct {
	Cycle []T
}

func (err ErrorCycle[T]) Error() string {
	return "Cycle found"
}

// NewEdge creates a new edge FROM -> TO, ensuring that FROM comes before TO in a traversal of g.
func (g *DAG[T]) NewEdge(from, to Node) error {
	// Self-loops are a no-op
	if from.i == to.i {
		return nil
	}

	// Check if adding edge from->to would create a cycle by checking if there's already
	// a path from 'to' to 'from' in the existing graph.
	// The adjacency list is maintained incrementally, so we use it directly for cycle detection.

	// DFS from 'to' to check if we can reach 'from' in the existing graph.
	// If we can, then adding 'from->to' would create a cycle.
	visited := make([]bool, len(g.nodes))
	parent := make([]int, len(g.nodes))
	for i := range parent {
		parent[i] = -1
	}

	// Stack-based DFS from 'to' following forward edges
	stack := []int{to.i}
	visited[to.i] = true

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		for _, neighbor := range g.nodes[current].to {
			if neighbor == from.i {
				// Cycle detected: there's already a path from 'to' to 'from'.
				// Adding 'from->to' would complete the cycle.
				// Reconstruct cycle as: from -> to -> ... -> current -> from

				// Build path from 'to' to 'current' (the node that reaches 'from')
				pathFromTo := []T{g.nodes[to.i].v}
				node := current
				for node != to.i {
					pathFromTo = append(pathFromTo, g.nodes[node].v)
					node = parent[node]
				}

				// Build cycle starting from 'from' following the path
				cycle := []T{g.nodes[from.i].v}
				cycle = append(cycle, pathFromTo...)
				return ErrorCycle[T]{Cycle: cycle}
			}

			if !visited[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = current
				stack = append(stack, neighbor)
			}
		}
	}

	// No cycle detected, we can now update the edge & adjacency list to include the
	// new edge.

	g.nodes[from.i].to = append(g.nodes[from.i].to, to.i)

	return nil
}

// Walk the DAG in parallel, blocking until all nodes have been processed or an error is
// returned.
//
// Walk does not mutate the [DAG] it's called on, and so is safe to call in parallel.
//
// It is not safe to add nodes or edges to the [DAG] while Walk is running.
func (g *DAG[T]) Walk(ctx context.Context, process func(context.Context, T) error, options ...WalkOption) error {
	var opts walkOptions
	for _, o := range options {
		o.applyWalkOption(&opts)
	}

	wg, ctx := errgroup.WithContext(ctx)
	if opts.maxProcs > 0 {
		wg.SetLimit(opts.maxProcs)
	}
	isDone := make([]chan struct{}, len(g.nodes))
	for i := range isDone {
		isDone[i] = make(chan struct{})
	}
	var sent sync.WaitGroup
	var didError atomic.Bool
	sent.Add(len(g.nodes))
	for i, node := range g.nodes {
		mustBefore := []chan struct{}{}
		for edge := range g.edges() {
			if edge.to == i {
				mustBefore = append(mustBefore, isDone[edge.from])
			}
		}
		go func() {
			for _, wait := range mustBefore {
				<-wait
			}
			wg.Go(func() error {
				defer close(isDone[i])
				if didError.Load() {
					return nil
				}
				err := process(ctx, node.v)
				didError.CompareAndSwap(false, err != nil)
				return err
			})
			sent.Done()
		}()
	}
	sent.Wait()
	return wg.Wait()
}

func (g *DAG[T]) edges() iter.Seq[edge] {
	return func(yield func(edge) bool) {
		for from, node := range g.nodes {
			for _, to := range node.to {
				if !yield(edge{
					from: from,
					to:   to,
				}) {
					return
				}
			}
		}
	}
}

type edge struct {
	from, to int
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
