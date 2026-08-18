// Package workflow provides an augmented view onto of [fsa] with richer semantics.
package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type Workflow struct {
	g fsa.FSA[cursor]
}

func New() Workflow {
	return Workflow{fsa.New[cursor]()}
}

func FromState(state json.RawMessage) Workflow {
	panic("TODO")
}

type NodeFunc = func(ctx context.Context, w Workflow, inputs property.Map) (property.Map, error)
type EdgeFunc = func(ctx context.Context, w Workflow, inputs property.Map) (bool, error)
type Node = fsa.Node
type Cursor struct {
	ID    string
	Label string
}

func (w Workflow) NewNode(id string, f NodeFunc) Node {
	panic("TODO")
}

func (w Workflow) NewEdge(name string, from, to Node, edge EdgeFunc) {
	panic("TODO")
}

func (w Workflow) NewOrEdge(name string, from, to Node, edges ...EdgeFunc) {
	panic("TODO")
}

func (w Workflow) NewAndEdge(name string, from, to Node, edges ...EdgeFunc) {
	panic("TODO")
}

func (w Workflow) NewJoinEdge(name string, from []JoinEdgeArg, to Node) {
	panic("TODO")
}

type JoinEdgeArg struct {
	From Node
	edge EdgeFunc
}

func (w Workflow) GetState(node Node) NodeState {
	panic("TODO")
}

func (w Workflow) Progress(
	ctx context.Context, runner fsa.Runner, updates chan<- WorkflowUpdate,
) error {
	return w.g.Progress(ctx, runner)
}

func (w Workflow) State() json.RawMessage {
	panic("TODO")
}

type NodeState struct {
	LastRun time.Time
	Inputs  property.Map
	Outputs property.Map
}

type cursor struct {
	value property.Map
}
