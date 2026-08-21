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

// Package workflow declares durable workflows as Pulumi resources.
//
// A workflow is a graph: nodes are Pulumi programs of their own, edges are conditions that let a cursor
// move from node to node, and cursors carry values through the graph. Every `pulumi up` advances the
// workflow once: each parked cursor's outgoing edges are asked, cursors that may move do so (running the
// node program they enter as a nested deployment), and every node that has ever run is reconciled again
// from the values its last cursor entered with. The engine keeps each node as a pulumi:index:WorkflowNode
// resource under the workflow, holding the node's occupant and visit history, with the node program's
// resources nested beneath it.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type (
	// A NodeFunc is a node's program. It runs as a nested Pulumi program when a cursor enters the node
	// and again on every up while the node exists, always from the values the cursor entered with; what it
	// sets on c becomes the values the cursor leaves with. Outputs set on c resolve when the program ends.
	NodeFunc = func(ctx *pulumi.Context, c *Cursor) error
	// An EdgeFunc decides whether c may cross an edge. It must be effect-free: it is asked at most once per
	// up for each parked cursor. What it sets on c applies only if the cursor crosses because of it.
	EdgeFunc = func(ctx context.Context, c *Cursor) (bool, error)
	// A PassFunc runs after a condition passes; see [EdgeMap.OnPass].
	PassFunc = func(ctx context.Context, c *Cursor) error
	// A MergeFunc decides a join once a cursor waits on each of its branches. Returning nil rejects the
	// merge and leaves every candidate where it is.
	MergeFunc = func(ctx context.Context, in []Candidate) (*Merged, error)
)

// An EdgeMap names the conditions of a composite edge.
type EdgeMap map[string]EdgeFunc

// OnPass wraps every condition so that f runs after it returns true. What f sets on the cursor rides that
// condition's decision: under [Context.And] f may run once per passing condition and its writes are
// discarded if the edge does not pass.
func (m EdgeMap) OnPass(f PassFunc) EdgeMap {
	out := make(EdgeMap, len(m))
	for name, cond := range m {
		out[name] = func(ctx context.Context, c *Cursor) (bool, error) {
			pass, err := cond(ctx, c)
			if err != nil || !pass {
				return pass, err
			}
			return true, f(ctx, c)
		}
	}
	return out
}

// A JoinMap names the nodes a join takes cursors from, each with the condition its cursor must pass (nil
// for none).
type JoinMap map[Node]EdgeFunc

// A Candidate is a cursor waiting on a join branch.
type Candidate struct {
	From   Node
	Cursor *Cursor
}

// Merged describes the cursor a join produces.
type Merged struct {
	Name   string
	Values map[string]any
}

// A Node of a workflow.
type Node struct{ name string }

// Name returns the node's name.
func (n Node) Name() string { return n.name }

// NodeState is what a callback may observe about a node.
type NodeState struct {
	Occupant string  // The name of the cursor on the node; empty if none.
	History  []Visit // The node's visits, newest first.
	LastRun  time.Time
}

// Previous returns the visit before the latest one.
func (s NodeState) Previous() (Visit, bool) {
	if len(s.History) < 2 {
		return Visit{}, false
	}
	return s.History[1], true
}

// A Visit is one stay of a cursor on a node.
type Visit struct {
	Cursor        string
	Entered, Left time.Time      // Left is zero while the cursor is still there.
	Inputs        map[string]any // The values the cursor entered with.
	Outputs       map[string]any // The values the node program produced; nil until it has run.
	Err           string         // The error of the program's last run, if it failed.
}

// A Context collects a workflow's definition.
type Context struct {
	nodes   map[string]NodeFunc
	order   []string
	edges   []edge
	entries map[string]entry
	err     error
}

type edge struct {
	name, from, to string
	kind           string
	conds          EdgeMap
	branches       JoinMap
	merge          MergeFunc
}

type entry struct {
	node   string
	inputs pulumi.Map
}

// Node defines a node. run may be nil for a waypoint: a node with no program.
func (w *Context) Node(name string, run NodeFunc) Node {
	if name == "" {
		w.fail(errors.New("workflow node names must not be empty"))
	} else if _, dup := w.nodes[name]; dup {
		w.fail(fmt.Errorf("workflow node %q defined twice", name))
	} else {
		w.nodes[name] = run
		w.order = append(w.order, name)
	}
	return Node{name}
}

// Cursor declares a cursor entry named name at node at. The engine places a cursor there whenever the
// resolved inputs differ from the last placement of name; an unchanged entry is a no-op. Cursors are
// named name#generation.
func (w *Context) Cursor(at Node, name string, inputs pulumi.Map) {
	if !w.defined(at) {
		return
	}
	if _, dup := w.entries[name]; dup {
		w.fail(fmt.Errorf("workflow entry %q defined twice", name))
		return
	}
	if inputs == nil {
		inputs = pulumi.Map{}
	}
	w.entries[name] = entry{node: at.name, inputs: inputs}
}

// Edge defines an edge from one node to another, taken when cond returns true. A node's outgoing edges
// are asked in definition order and the first to pass is taken.
func (w *Context) Edge(name string, from, to Node, cond EdgeFunc) {
	w.addEdge(edge{name: name, from: from.name, to: to.name, kind: "single", conds: EdgeMap{name: cond}}, from, to)
}

// And defines an edge taken when every condition returns true. Conditions are asked together; their
// writes merge, the first in the order of their names winning a key set by several.
func (w *Context) And(name string, from, to Node, conds EdgeMap) {
	w.addEdge(edge{name: name, from: from.name, to: to.name, kind: "and", conds: conds}, from, to)
}

// Or defines an edge taken when any condition returns true. Conditions are asked one at a time in the
// order of their names and the first true wins; only its writes apply.
func (w *Context) Or(name string, from, to Node, conds EdgeMap) {
	w.addEdge(edge{name: name, from: from.name, to: to.name, kind: "or", conds: conds}, from, to)
}

func (w *Context) addEdge(e edge, from, to Node) {
	if !w.defined(from) || !w.defined(to) {
		return
	}
	if len(e.conds) == 0 {
		w.fail(fmt.Errorf("workflow edge %q has no conditions", e.name))
		return
	}
	for cond, f := range e.conds {
		if f == nil {
			w.fail(fmt.Errorf("workflow edge %q: condition %q is nil", e.name, cond))
			return
		}
	}
	w.edges = append(w.edges, e)
}

// Join defines an edge that merges one cursor from each node of from into a single cursor at to. The join
// is taken only when every from node holds a cursor and, evaluated together at that moment, every branch's
// condition passes and merge accepts; otherwise every cursor stays where it is. Join names must be unique.
func (w *Context) Join(name string, from JoinMap, to Node, merge MergeFunc) {
	if !w.defined(to) {
		return
	}
	if len(from) == 0 {
		w.fail(fmt.Errorf("workflow join %q has no branches", name))
		return
	}
	for n := range from {
		if !w.defined(n) {
			return
		}
		if n == to {
			w.fail(fmt.Errorf("workflow join %q: a join cannot target one of its own nodes", name))
			return
		}
	}
	if merge == nil {
		w.fail(fmt.Errorf("workflow join %q has no merge function", name))
		return
	}
	for _, e := range w.edges {
		if e.kind == "join" && e.name == name {
			w.fail(fmt.Errorf("workflow join %q defined twice", name))
			return
		}
	}
	w.edges = append(w.edges, edge{name: name, to: to.name, kind: "join", branches: from, merge: merge})
}

// AtNode reports the state of node n as of the moment the callback whose context is ctx was dispatched.
// It panics if ctx does not come from a workflow callback (in a node program, pass ctx.Context()).
func (w *Context) AtNode(ctx context.Context, n Node) NodeState {
	v, ok := ctx.Value(viewKey{}).(*pulumirpc.WorkflowView)
	if !ok {
		panic("workflow: AtNode must be called with the context of a workflow callback")
	}
	return nodeState(v.GetNodes()[n.name])
}

type viewKey struct{}

func (w *Context) defined(n Node) bool {
	if _, ok := w.nodes[n.name]; ok {
		return true
	}
	w.fail(fmt.Errorf("workflow node %q is not defined in this workflow", n.name))
	return false
}

func (w *Context) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// Workflow is the workflow resource.
type Workflow struct {
	pulumi.CustomResourceState

	// The cursors of the workflow after the last up: objects with a "name" and a "node".
	Cursors pulumi.ArrayOutput `pulumi:"cursors"`
	// A rendering of the workflow after the last up.
	Diagram pulumi.StringOutput `pulumi:"diagram"`
}

// New registers a workflow resource whose definition def builds.
func New(ctx *pulumi.Context, name string, def func(*Context) error, opts ...pulumi.ResourceOption) (*Workflow, error) {
	w := &Context{nodes: map[string]NodeFunc{}, entries: map[string]entry{}}
	if err := def(w); err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}

	callback := func(fn func(context.Context, []byte) (proto.Message, error)) (pulumi.Map, error) {
		cb, err := registerCallback(ctx, fn)
		if err != nil {
			return nil, err
		}
		return pulumi.Map{"target": pulumi.String(cb.Target), "token": pulumi.String(cb.Token)}, nil
	}

	nodes := pulumi.Map{}
	for _, n := range w.order {
		node := pulumi.Map{}
		if run := w.nodes[n]; run != nil {
			cb, err := callback(nodeCallback(w, run))
			if err != nil {
				return nil, err
			}
			node["program"] = cb
		}
		nodes[n] = node
	}

	edges := make(pulumi.Array, 0, len(w.edges))
	for _, e := range w.edges {
		m := pulumi.Map{"name": pulumi.String(e.name), "to": pulumi.String(e.to), "kind": pulumi.String(e.kind)}
		switch e.kind {
		case "join":
			branches := pulumi.Map{}
			for n, cond := range e.branches {
				if cond == nil {
					branches[n.name] = nil
					continue
				}
				cb, err := callback(conditionCallback(w, cond))
				if err != nil {
					return nil, err
				}
				branches[n.name] = cb
			}
			cb, err := callback(mergeCallback(w, e.merge))
			if err != nil {
				return nil, err
			}
			m["branches"], m["merge"] = branches, cb
		default:
			conds := pulumi.Map{}
			for c, cond := range e.conds {
				cb, err := callback(conditionCallback(w, cond))
				if err != nil {
					return nil, err
				}
				conds[c] = cb
			}
			m["from"], m["conditions"] = pulumi.String(e.from), conds
		}
		edges = append(edges, m)
	}

	entries := pulumi.Map{}
	for n, e := range w.entries {
		entries[n] = pulumi.Map{"node": pulumi.String(e.node), "inputs": e.inputs}
	}

	var wf Workflow
	err := ctx.RegisterResource("pulumi:index:Workflow", name,
		pulumi.Map{"nodes": nodes, "edges": edges, "entries": entries}, &wf, opts...)
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

// -- Callbacks --

var valueMarshalOptions = plugin.MarshalOptions{KeepSecrets: true, KeepResources: true, KeepOutputValues: true}

func decodeValues(s *structpb.Struct) (resource.PropertyMap, error) {
	if s == nil {
		return resource.PropertyMap{}, nil
	}
	return plugin.UnmarshalProperties(s, valueMarshalOptions)
}

func encodeValues(m resource.PropertyMap) (*structpb.Struct, error) {
	return plugin.MarshalProperties(m, valueMarshalOptions)
}

// guarded runs fn, turning a panic (such as Require's) into an error so the engine reports it.
func guarded[T any](fn func() (T, error)) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("workflow callback panicked: %v", r)
		}
	}()
	return fn()
}

// nodeCallback runs a node program under a fresh Pulumi context bound to the nested resource monitor the
// engine started for this run, and answers with the values the cursor leaves the node with.
func nodeCallback(w *Context, run NodeFunc) func(context.Context, []byte) (proto.Message, error) {
	return func(cbCtx context.Context, b []byte) (proto.Message, error) {
		return guarded(func() (proto.Message, error) {
			var req pulumirpc.WorkflowNodeRequest
			if err := proto.Unmarshal(b, &req); err != nil {
				return nil, fmt.Errorf("invalid workflow node request: %w", err)
			}
			values, err := decodeValues(req.GetCursor().GetValues())
			if err != nil {
				return nil, err
			}
			c := newCursor(req.GetCursor().GetName(), values, true)
			info := pulumi.RunInfo{
				Project:          req.Project,
				Stack:            req.Stack,
				Organization:     req.Organization,
				Config:           req.Config,
				ConfigSecretKeys: req.ConfigSecretKeys,
				Parallel:         req.Parallel,
				MonitorAddr:      req.MonitorAddr,
				EngineAddr:       req.EngineAddr,
			}
			base := context.WithValue(context.WithoutCancel(cbCtx), viewKey{}, req.View)
			nested, err := pulumi.NewContext(base, info)
			if err != nil {
				return nil, fmt.Errorf("creating workflow node context: %w", err)
			}
			defer nested.Close()
			if err := pulumi.RunWithContext(nested, func(ctx *pulumi.Context) error { return run(ctx, c) }); err != nil {
				return nil, err
			}
			outgoing, err := c.outgoing()
			if err != nil {
				return nil, err
			}
			s, err := encodeValues(outgoing)
			if err != nil {
				return nil, err
			}
			return &pulumirpc.WorkflowNodeResponse{Outputs: s}, nil
		})
	}
}

func conditionCallback(w *Context, cond EdgeFunc) func(context.Context, []byte) (proto.Message, error) {
	return func(cbCtx context.Context, b []byte) (proto.Message, error) {
		return guarded(func() (proto.Message, error) {
			var req pulumirpc.WorkflowConditionRequest
			if err := proto.Unmarshal(b, &req); err != nil {
				return nil, fmt.Errorf("invalid workflow condition request: %w", err)
			}
			values, err := decodeValues(req.GetCursor().GetValues())
			if err != nil {
				return nil, err
			}
			c := newCursor(req.GetCursor().GetName(), values, false)
			pass, err := cond(context.WithValue(cbCtx, viewKey{}, req.View), c)
			if err != nil {
				return nil, err
			}
			overlay, err := c.overlay()
			if err != nil {
				return nil, err
			}
			s, err := encodeValues(overlay)
			if err != nil {
				return nil, err
			}
			return &pulumirpc.WorkflowConditionResponse{Pass: pass, Overlay: s}, nil
		})
	}
}

func mergeCallback(w *Context, merge MergeFunc) func(context.Context, []byte) (proto.Message, error) {
	return func(cbCtx context.Context, b []byte) (proto.Message, error) {
		return guarded(func() (proto.Message, error) {
			var req pulumirpc.WorkflowMergeRequest
			if err := proto.Unmarshal(b, &req); err != nil {
				return nil, fmt.Errorf("invalid workflow merge request: %w", err)
			}
			candidates := make([]Candidate, len(req.Candidates))
			for i, cand := range req.Candidates {
				values, err := decodeValues(cand.GetCursor().GetValues())
				if err != nil {
					return nil, err
				}
				candidates[i] = Candidate{From: Node{cand.From}, Cursor: newCursor(cand.GetCursor().GetName(), values, false)}
			}
			merged, err := merge(context.WithValue(cbCtx, viewKey{}, req.View), candidates)
			if err != nil {
				return nil, err
			}
			if merged == nil {
				return &pulumirpc.WorkflowMergeResponse{}, nil
			}
			values := resource.PropertyMap{}
			for k, v := range merged.Values {
				pv, err := marshalPlain(v)
				if err != nil {
					return nil, fmt.Errorf("merged value %q: %w", k, err)
				}
				values[resource.PropertyKey(k)] = pv
			}
			s, err := encodeValues(values)
			if err != nil {
				return nil, err
			}
			return &pulumirpc.WorkflowMergeResponse{Merge: true, Name: merged.Name, Values: s}, nil
		})
	}
}

// outgoing computes the values a cursor leaves a node with: what it carried, with this program's writes.
func (c *Cursor) outgoing() (resource.PropertyMap, error) {
	c.m.Lock()
	defer c.m.Unlock()
	out := c.values.Copy()
	for key := range c.deleted {
		delete(out, resource.PropertyKey(key))
	}
	for key, v := range c.sets {
		pv, err := marshalValue(v)
		if err != nil {
			return nil, fmt.Errorf("value %q: %w", key, err)
		}
		out[resource.PropertyKey(key)] = pv
	}
	return out, nil
}

// overlay computes what an edge callback wrote.
func (c *Cursor) overlay() (resource.PropertyMap, error) {
	c.m.Lock()
	defer c.m.Unlock()
	if len(c.deleted) > 0 {
		return nil, fmt.Errorf("Delete(%q) is only supported in a node program", slices.Sorted(maps.Keys(c.deleted))[0])
	}
	out := resource.PropertyMap{}
	for key, v := range c.sets {
		pv, err := marshalPlain(v)
		if err != nil {
			return nil, fmt.Errorf("value %q: %w", key, err)
		}
		out[resource.PropertyKey(key)] = pv
	}
	return out, nil
}

// marshalPlain marshals a value that may not be an Output.
func marshalPlain(v any) (resource.PropertyValue, error) {
	if _, ok := v.(pulumi.Output); ok {
		return resource.PropertyValue{}, errors.New("an Output may only be set in a node program")
	}
	return marshalValue(v)
}

func nodeState(s *pulumirpc.WorkflowNodeState) NodeState {
	if s == nil {
		return NodeState{}
	}
	state := NodeState{Occupant: s.Occupant}
	if s.LastRun != nil {
		state.LastRun = s.LastRun.AsTime()
	}
	for _, v := range s.History {
		visit := Visit{Cursor: v.Cursor, Err: v.Error}
		if v.Entered != nil {
			visit.Entered = v.Entered.AsTime()
		}
		if v.Left != nil {
			visit.Left = v.Left.AsTime()
		}
		visit.Inputs = goValues(v.Inputs)
		if v.Outputs != nil {
			visit.Outputs = goValues(v.Outputs)
		}
		state.History = append(state.History, visit)
	}
	return state
}

func goValues(s *structpb.Struct) map[string]any {
	m, err := decodeValues(s)
	if err != nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[string(k)] = toGo(v)
	}
	return out
}
