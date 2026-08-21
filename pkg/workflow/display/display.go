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

// Package display folds the event stream of [workflow] into a plain-text diagram.
package display

import (
	"cmp"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/workflow"
)

// A Model is the state of a workflow as told by its event stream.
type Model struct {
	nodes     []string // In order of first sight
	undefined map[string]bool
	edges     map[string][]workflow.EdgeDefined // Outgoing edges by from node, in definition order
	cursors   []*cursor                         // In order of first appearance
}

type cursor struct {
	id, label string
	node      string
	running   bool
	checking  []workflow.EdgeIdentity // Edges asked of the cursor and not yet answered
	crossing  string                  // The target of the last passed edge, until the cursor enters a node
	err       error
}

func New() *Model {
	return &Model{undefined: map[string]bool{}, edges: map[string][]workflow.EdgeDefined{}}
}

// Apply folds one event into the model. Safe to call from one goroutine at a time.
func (m *Model) Apply(u workflow.WorkflowUpdate) {
	switch u := u.(type) {
	case workflow.NodeDefined:
		m.node(u.ID)
		delete(m.undefined, u.ID)
	case workflow.NodeUndefined:
		m.node(u.ID)
		m.undefined[u.ID] = true
		for _, c := range u.Cursors {
			m.cursor(c).node = u.ID
		}
	case workflow.EdgeDefined:
		if len(u.JoinEdges) > 0 {
			for _, b := range u.JoinEdges {
				m.edges[b.From.ID()] = append(m.edges[b.From.ID()], u)
			}
		} else {
			m.edges[u.From.ID()] = append(m.edges[u.From.ID()], u)
		}
	case workflow.CursorAdded:
		m.cursor(u.Cursor).node = u.Node.ID()
	case workflow.CursorReplaced:
		m.remove(u.Old)
	case workflow.CursorsJoined:
		c := m.cursor(u.New)
		for _, old := range u.Old {
			if o := m.find(old); o != nil && (o.crossing != "" || c.node == "") {
				c.node = cmp.Or(o.crossing, o.node)
			}
			m.remove(old)
		}
	case workflow.NodeStarted:
		c := m.cursor(u.Cursor)
		c.node, c.running, c.crossing = u.ID, true, ""
	case workflow.NodeSucceeded:
		m.cursor(u.Cursor).running = false
	case workflow.NodeFailed:
		c := m.cursor(u.Cursor)
		c.running, c.err = false, u.Error
	case workflow.EdgeStarted:
		c := m.cursor(u.Cursor)
		c.checking = append(c.checking, u.EdgeIdentity)
	case workflow.EdgeFinished:
		c := m.cursor(u.Cursor)
		c.answered(u.EdgeIdentity)
		if u.Pass {
			c.crossing = u.To.ID()
		}
	case workflow.EdgeFailed:
		c := m.cursor(u.Cursor)
		c.answered(u.EdgeIdentity)
		c.err = u.Error
	}
}

func (c *cursor) answered(e workflow.EdgeIdentity) {
	if i := slices.Index(c.checking, e); i >= 0 {
		c.checking = slices.Delete(c.checking, i, i+1)
	}
}

func (m *Model) node(id string) {
	if !slices.Contains(m.nodes, id) {
		m.nodes = append(m.nodes, id)
	}
}

func (m *Model) find(c workflow.Cursor) *cursor {
	if i := slices.IndexFunc(m.cursors, func(x *cursor) bool { return x.id == c.ID }); i >= 0 {
		return m.cursors[i]
	}
	return nil
}

func (m *Model) cursor(c workflow.Cursor) *cursor {
	if x := m.find(c); x != nil {
		return x
	}
	x := &cursor{id: c.ID, label: c.Label}
	m.cursors = append(m.cursors, x)
	return x
}

func (m *Model) remove(c workflow.Cursor) {
	m.cursors = slices.DeleteFunc(m.cursors, func(x *cursor) bool { return x.id == c.ID })
}

// Render draws the model: the graph, then one line per cursor.
func (m *Model) Render() string {
	var b strings.Builder
	b.WriteString("workflow\n")
	for _, id := range m.nodes {
		b.WriteString("  " + id)
		if m.undefined[id] {
			b.WriteString(" (undefined)")
		}
		var occupants []string
		for _, c := range m.cursors {
			if c.node == id {
				occupants = append(occupants, c.name())
			}
		}
		if len(occupants) > 0 {
			b.WriteString(" ◀ " + strings.Join(occupants, ", "))
		}
		b.WriteString("\n")
		for i, e := range m.edges[id] {
			branch := "├─ "
			if i == len(m.edges[id])-1 {
				branch = "└─ "
			}
			b.WriteString("    " + branch + describe(e) + " ─▶ " + e.To.ID() + "\n")
		}
	}
	b.WriteString("cursors\n")
	for _, c := range m.cursors {
		b.WriteString("  " + c.name())
		if c.node != "" {
			b.WriteString(" @ " + c.node)
		}
		b.WriteString(" — " + c.status())
		if c.err != nil {
			b.WriteString(" (error: " + c.err.Error() + ")")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func describe(e workflow.EdgeDefined) string {
	var kind string
	children, name := e.AndEdges, func(c workflow.EdgeDefined) string { return c.Condition }
	switch {
	case len(e.AndEdges) > 0:
		kind = "all"
	case len(e.OrEdges) > 0:
		kind, children = "any", e.OrEdges
	case len(e.JoinEdges) > 0:
		kind, children = "join", e.JoinEdges
		name = func(c workflow.EdgeDefined) string { return c.From.ID() }
	default:
		return e.Name
	}
	names := make([]string, len(children))
	for i, c := range children {
		names[i] = name(c)
	}
	return e.Name + " (" + kind + ": " + strings.Join(names, ", ") + ")"
}

func (c *cursor) name() string {
	if c.label != "" {
		return c.label
	}
	return c.id
}

func (c *cursor) status() string {
	switch {
	case c.running:
		return "running " + c.node
	case len(c.checking) > 0:
		e := c.checking[len(c.checking)-1]
		if e.Condition != "" {
			return "checking " + e.Name + "/" + e.Condition
		}
		return "checking " + e.Name
	}
	return "parked"
}
