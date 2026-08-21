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

type NodeStarted struct {
	ID     string       `json:"id"`
	Inputs property.Map `json:"inputs"`
}

func (e NodeStarted) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID     string `json:"id"`
		Inputs any    `json:"inputs"`
	}{e.ID, propertyJSON(e.Inputs)})
}

type NodeSucceeded struct {
	ID      string       `json:"id"`
	Outputs property.Map `json:"outputs"`
}

func (e NodeSucceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID      string `json:"id"`
		Outputs any    `json:"outputs"`
	}{e.ID, propertyJSON(e.Outputs)})
}

type NodeFailed struct {
	ID    string `json:"id"`
	Error error  `json:"error"`
}

func (e NodeFailed) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	}{e.ID, e.Error.Error()})
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

type EdgeIdentity struct {
	Name string `json:"name"`
	From Node   `json:"from"`
	To   Node   `json:"to"`
}

type EdgeStarted struct {
	EdgeIdentity
	Inputs property.Map `json:"inputs"`
}

func (e EdgeStarted) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		EdgeIdentity
		Inputs any `json:"inputs"`
	}{e.EdgeIdentity, propertyJSON(e.Inputs)})
}

type EdgeFinished struct {
	EdgeIdentity
	Pass bool `json:"pass"`
}

type EdgeFailed struct {
	EdgeIdentity
	Error error `json:"error"`
}

func (e EdgeFailed) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		EdgeIdentity
		Error string `json:"error"`
	}{e.EdgeIdentity, e.Error.Error()})
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
