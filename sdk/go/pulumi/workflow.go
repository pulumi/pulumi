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

package pulumi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// A WorkflowGraph describes a workflow: nodes are mini Pulumi programs with their own persistent
// sub-state, edges are condition functions that gate cursor movement, and entries admit cursors
// carrying data through the graph. Build one inside the definition callback passed to NewWorkflow.
type WorkflowGraph struct {
	nodes   []*WorkflowNode
	edges   []workflowGraphEdge
	entries map[string]Map
	err     error
}

// A WorkflowNode is a named mini Pulumi program within a workflow. Its program runs (as a nested
// deployment against the node's own sub-state) when a cursor enters the node, and again on every
// up while a cursor occupies it. Cursor data is exposed to the program as config under the
// "workflow" namespace; the program's stack exports flow into the cursor as it leaves.
type WorkflowNode struct {
	name string
	fn   RunFunc
}

// A WorkflowCondition guards an edge. It is sampled at most once per cursor visit — every up
// starts a fresh visit — and must be effect-free. Use WorkflowFrom to read the traversing
// cursor's data, arrival time, and fingerprint.
type WorkflowCondition = func(context.Context) (bool, error)

type workflowGraphEdge struct {
	from, to *WorkflowNode
	cond     WorkflowCondition
}

// DefNode defines a node with the given name and program.
func (g *WorkflowGraph) DefNode(name string, program RunFunc) *WorkflowNode {
	for _, n := range g.nodes {
		if n.name == name {
			g.fail(fmt.Errorf("workflow node %q defined twice", name))
		}
	}
	n := &WorkflowNode{name: name, fn: program}
	g.nodes = append(g.nodes, n)
	return n
}

// Edge defines an edge from one node to another, guarded by cond. Edges are considered in
// definition order and the first passing edge wins, so put exceptional edges (rollback, exit)
// before the normal path.
func (g *WorkflowGraph) Edge(from, to *WorkflowNode, cond WorkflowCondition) {
	if from == nil || to == nil {
		g.fail(errors.New("workflow edge endpoints must be defined nodes"))
		return
	}
	g.edges = append(g.edges, workflowGraphEdge{from: from, to: to, cond: cond})
}

// Entry declares n's entry slot. Each node has one: a seed whose resolved value differs from the
// last admitted one admits a new cursor at n (without running n's program); an unchanged seed is a
// no-op. Include a distinguishing field (a run id, an artifact version) to re-run a workflow with
// an otherwise identical payload.
func (g *WorkflowGraph) Entry(n *WorkflowNode, seed Map) {
	if n == nil {
		g.fail(errors.New("workflow entry must name a defined node"))
		return
	}
	if _, ok := g.entries[n.name]; ok {
		g.fail(fmt.Errorf("workflow node %q has more than one entry", n.name))
		return
	}
	g.entries[n.name] = seed
}

func (g *WorkflowGraph) fail(err error) {
	if g.err == nil {
		g.err = err
	}
}

// WorkflowAlways is an edge condition that always passes.
func WorkflowAlways(context.Context) (bool, error) { return true, nil }

// A WorkflowTraversal describes the cursor a condition is being evaluated for.
type WorkflowTraversal struct {
	// When the cursor entered the edge's source node. Enables bake-time gates.
	When time.Time
	// Fingerprint is a stable hash of the cursor's data. Approvals should bind to it: any change
	// to the cursor's data changes the fingerprint, structurally invalidating prior approvals.
	Fingerprint string

	data map[string]any
}

// Data returns the traversing cursor's data.
func (t WorkflowTraversal) Data() map[string]any { return t.data }

type workflowTraversalKey struct{}

// WorkflowFrom returns the traversal a condition is being evaluated for. It panics if ctx does not
// originate from a workflow condition invocation.
func WorkflowFrom(ctx context.Context) WorkflowTraversal {
	t, ok := ctx.Value(workflowTraversalKey{}).(WorkflowTraversal)
	if !ok {
		panic("WorkflowFrom must be called from a workflow condition")
	}
	return t
}

// Workflow is a durable workflow resource. Its state holds the cursors and each node's sub-state.
type Workflow struct {
	CustomResourceState
}

// NewWorkflow registers a workflow resource whose graph is built by def.
//
// The workflow only advances during an up: each up polls every parked cursor's untried edge
// conditions once, runs entered nodes' programs as nested deployments, and re-reconciles every
// node currently hosting a cursor. Nodes removed from the definition have their sub-state
// destroyed; destroying the workflow destroys every node's sub-state.
func NewWorkflow(ctx *Context, name string, def func(*WorkflowGraph) error,
	opts ...ResourceOption,
) (*Workflow, error) {
	g := &WorkflowGraph{entries: map[string]Map{}}
	if err := def(g); err != nil {
		return nil, err
	}
	if g.err != nil {
		return nil, g.err
	}

	nodes := Map{}
	for _, n := range g.nodes {
		if n.fn == nil {
			return nil, fmt.Errorf("workflow node %q has no program", n.name)
		}
		cb, err := ctx.registerWorkflowCallback(workflowNodeCallback(ctx, n.fn))
		if err != nil {
			return nil, err
		}
		nodes[n.name] = Map{"target": String(cb.Target), "token": String(cb.Token)}
	}

	edges := make(Array, len(g.edges))
	for i, e := range g.edges {
		if e.cond == nil {
			return nil, fmt.Errorf("workflow edge %s -> %s has no condition", e.from.name, e.to.name)
		}
		cb, err := ctx.registerWorkflowCallback(workflowConditionCallback(e.cond))
		if err != nil {
			return nil, err
		}
		edges[i] = Map{
			"from":   String(e.from.name),
			"to":     String(e.to.name),
			"target": String(cb.Target),
			"token":  String(cb.Token),
		}
	}

	entries := Map{}
	for name, seed := range g.entries {
		entries[name] = seed
	}

	var wf Workflow
	err := ctx.RegisterResource("pulumi:index:Workflow", name,
		Map{"nodes": nodes, "edges": edges, "entries": entries}, &wf, opts...)
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

// registerWorkflowCallback registers fn with the context's callback server, starting the server if
// necessary.
func (ctx *Context) registerWorkflowCallback(fn callbackFunction) (*pulumirpc.Callback, error) {
	err := func() error {
		ctx.state.callbacksLock.Lock()
		defer ctx.state.callbacksLock.Unlock()
		if ctx.state.callbacks == nil {
			c, err := newCallbackServer()
			if err != nil {
				return fmt.Errorf("creating callback server: %w", err)
			}
			ctx.state.callbacks = c
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	return ctx.state.callbacks.RegisterCallback(fn)
}

// workflowNodeRequest mirrors the engine's payload for running a node program.
type workflowNodeRequest struct {
	MonitorAddr string            `json:"monitorAddr"`
	Project     string            `json:"project"`
	Stack       string            `json:"stack"`
	Config      map[string]string `json:"config"`
	Parallel    int32             `json:"parallel"`
}

// workflowNodeCallback runs a node's program under a fresh Pulumi context bound to the nested
// resource monitor the engine started for this node reconcile.
func workflowNodeCallback(outer *Context, fn RunFunc) callbackFunction {
	return func(cbCtx context.Context, req []byte) (proto.Message, error) {
		var p workflowNodeRequest
		if err := json.Unmarshal(req, &p); err != nil {
			return nil, fmt.Errorf("invalid workflow node request: %w", err)
		}
		info := RunInfo{
			Project:     p.Project,
			Stack:       p.Stack,
			Config:      p.Config,
			Parallel:    p.Parallel,
			MonitorAddr: p.MonitorAddr,
			EngineAddr:  outer.state.info.EngineAddr,
		}
		nested, err := NewContext(context.WithoutCancel(cbCtx), info)
		if err != nil {
			return nil, fmt.Errorf("creating workflow node context: %w", err)
		}
		defer nested.Close()
		if err := RunWithContext(nested, fn); err != nil {
			return nil, err
		}
		return wrapperspb.String("{}"), nil
	}
}

// workflowConditionRequest mirrors the engine's payload for evaluating an edge condition.
type workflowConditionRequest struct {
	Data        map[string]any `json:"data"`
	When        time.Time      `json:"when"`
	Fingerprint string         `json:"fingerprint"`
}

func workflowConditionCallback(cond WorkflowCondition) callbackFunction {
	return func(cbCtx context.Context, req []byte) (proto.Message, error) {
		var p workflowConditionRequest
		if err := json.Unmarshal(req, &p); err != nil {
			return nil, fmt.Errorf("invalid workflow condition request: %w", err)
		}
		t := WorkflowTraversal{When: p.When, Fingerprint: p.Fingerprint, data: p.Data}
		pass, err := cond(context.WithValue(cbCtx, workflowTraversalKey{}, t))
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(map[string]bool{"pass": pass})
		if err != nil {
			return nil, err
		}
		return wrapperspb.String(string(b)), nil
	}
}
