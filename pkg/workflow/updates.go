package workflow

import "github.com/pulumi/pulumi/sdk/v3/go/property"

type WorkflowUpdate interface {
	isWorkflowUpdate()
}

func (NodeStarted) isWorkflowUpdate()   {}
func (NodeSucceeded) isWorkflowUpdate() {}
func (NodeFailed) isWorkflowUpdate()    {}
func (EdgeStarted) isWorkflowUpdate()   {}
func (EdgeFinished) isWorkflowUpdate()  {}
func (EdgeFailed) isWorkflowUpdate()    {}
func (NodeDefined) isWorkflowUpdate()   {}
func (EdgeDefined) isWorkflowUpdate()   {}

type NodeStarted struct {
	ID     string
	Inputs property.Map
}

type NodeSucceeded struct {
	ID      string
	Outputs property.Map
}

type NodeFailed struct {
	ID    string
	Error error
}

type NodeDefined struct {
	ID string
}

type EdgeDefined struct {
	EdgeIdentity

	// May be a simple edge definition, or one of the following may have length > 0.

	AndEdges  []EdgeDefined
	OrEdges   []EdgeDefined
	JoinEdges []EdgeDefined
}

type EdgeIdentity struct {
	Name string
	From Node
	To   Node
}

type EdgeStarted struct {
	EdgeIdentity
	Inputs property.Map
}

type EdgeFinished struct {
	EdgeIdentity
	Pass bool
}

type EdgeFailed struct {
	EdgeIdentity
	Error error
}

type CursorAdded struct {
	Node   Node
	Inputs property.Map
}

type CursorReplaced struct {
	Old  Cursor
	New  Cursor
	Node Node
}

type CursorsJoined struct {
	Old []Cursor
	New Cursor
}
