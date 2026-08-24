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

package workflow

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type WorkflowUpdate interface {
	isWorkflowUpdate()
}

func (NodeStarted) isWorkflowUpdate()    {}
func (NodeSucceeded) isWorkflowUpdate()  {}
func (NodeFailed) isWorkflowUpdate()     {}
func (EdgeStarted) isWorkflowUpdate()    {}
func (EdgeFinished) isWorkflowUpdate()   {}
func (EdgeFailed) isWorkflowUpdate()     {}
func (NodeDefined) isWorkflowUpdate()    {}
func (EdgeDefined) isWorkflowUpdate()    {}
func (CursorAdded) isWorkflowUpdate()    {}
func (CursorsJoined) isWorkflowUpdate()  {}
func (NodeUndefined) isWorkflowUpdate()  {}
func (CursorReplaced) isWorkflowUpdate() {}
func (NodeUntouched) isWorkflowUpdate()  {}

// NodeStarted reports that node ID's function started running for Cursor, which entered with Inputs.
type NodeStarted struct {
	ID     string       `json:"id"`
	Cursor Cursor       `json:"cursor"`
	Inputs property.Map `json:"inputs"`
}

func (e NodeStarted) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID     string `json:"id"`
		Cursor Cursor `json:"cursor"`
		Inputs any    `json:"inputs"`
	}{e.ID, e.Cursor, propertyJSON(e.Inputs)})
}

// NodeSucceeded reports that node ID's function produced Outputs, the values Cursor leaves the node with.
type NodeSucceeded struct {
	ID      string       `json:"id"`
	Cursor  Cursor       `json:"cursor"`
	Outputs property.Map `json:"outputs"`
}

func (e NodeSucceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID      string `json:"id"`
		Cursor  Cursor `json:"cursor"`
		Outputs any    `json:"outputs"`
	}{e.ID, e.Cursor, propertyJSON(e.Outputs)})
}

type NodeFailed struct {
	ID     string `json:"id"`
	Cursor Cursor `json:"cursor"`
	Error  error  `json:"error"`
}

func (e NodeFailed) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID     string `json:"id"`
		Cursor Cursor `json:"cursor"`
		Error  string `json:"error"`
	}{e.ID, e.Cursor, e.Error.Error()})
}

type NodeDefined struct {
	ID string `json:"id"`
}

// NodeUndefined reports, at the end of a Progress, a node that restored cursors sit on but that has not
// been defined. The cursors stay put until it is.
type NodeUndefined struct {
	ID      string   `json:"id"`
	Cursors []Cursor `json:"cursors"`
}

// NodeUntouched reports, at the end of a Progress, a defined node whose function did not run.
type NodeUntouched struct {
	ID string `json:"id"`
}

type EdgeDefined struct {
	EdgeIdentity

	// May be a simple edge definition, or one of the following may have length > 0.

	AndEdges  []EdgeDefined `json:"andEdges,omitempty"`
	OrEdges   []EdgeDefined `json:"orEdges,omitempty"`
	JoinEdges []EdgeDefined `json:"joinEdges,omitempty"`
}

// EdgeIdentity names an edge; for one condition of a composite edge, Condition names it within the edge.
type EdgeIdentity struct {
	Name      string `json:"name"`
	Condition string `json:"condition,omitempty"`
	From      Node   `json:"from"`
	To        Node   `json:"to"`
}

// EdgeStarted reports that an edge (or one of its conditions) is being asked of Cursor, whose node
// produced Inputs.
type EdgeStarted struct {
	EdgeIdentity
	Cursor Cursor       `json:"cursor"`
	Inputs property.Map `json:"inputs"`
}

func (e EdgeStarted) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		EdgeIdentity
		Cursor Cursor `json:"cursor"`
		Inputs any    `json:"inputs"`
	}{e.EdgeIdentity, e.Cursor, propertyJSON(e.Inputs)})
}

// EdgeFinished reports an edge's (or condition's) answer for Cursor. Overlay applies to the cursor's values
// if it crosses the edge because of this answer.
type EdgeFinished struct {
	EdgeIdentity
	Cursor  Cursor  `json:"cursor"`
	Pass    bool    `json:"pass"`
	Overlay Overlay `json:"overlay"`
}

type EdgeFailed struct {
	EdgeIdentity
	Cursor Cursor `json:"cursor"`
	Error  error  `json:"error"`
}

func (e EdgeFailed) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		EdgeIdentity
		Cursor Cursor `json:"cursor"`
		Error  string `json:"error"`
	}{e.EdgeIdentity, e.Cursor, e.Error.Error()})
}

type CursorAdded struct {
	Node   Node         `json:"node"`
	Cursor Cursor       `json:"cursor"`
	Inputs property.Map `json:"inputs"`
}

func (e CursorAdded) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Node   Node   `json:"node"`
		Cursor Cursor `json:"cursor"`
		Inputs any    `json:"inputs"`
	}{e.Node, e.Cursor, propertyJSON(e.Inputs)})
}

// CursorReplaced reports that Old, unable to move on from Node, was overwritten by New arriving there.
// Every cursor ends in exactly one CursorReplaced or CursorsJoined.
type CursorReplaced struct {
	Old  Cursor `json:"old"`
	New  Cursor `json:"new"`
	Node Node   `json:"node"`
}

type CursorsJoined struct {
	Old []Cursor `json:"old"`
	New Cursor   `json:"new"`
}

func propertyJSON(m property.Map) any { return resource.ToResourcePropertyMap(m).Mappable() }
